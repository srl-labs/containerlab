// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabcert "github.com/srl-labs/containerlab/cert"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	// container validation variables.
	containerNamePattern = "^[a-zA-Z0-9][a-zA-Z0-9-._]+$"
	dnsIncompatibleChars = "._"
	maxNameLength        = 60
)

var containerNamePatternRe = regexp.MustCompile(containerNamePattern)

// DefaultNode implements the Node interface and is embedded to the structs of all other nodes.
// It has common fields and methods that every node should typically have. Nodes can override
// methods if needed.
type DefaultNode struct {
	Cfg              *clabtypes.NodeConfig
	Mgmt             *clabtypes.MgmtNet
	Runtime          clabruntime.ContainerRuntime
	HostRequirements *clabtypes.HostRequirements
	// SSHConfig is the SSH client configuration that a clab node requires.
	SSHConfig *clabtypes.SSHConfig
	// Indicates that the node should not start without no license file defined
	LicensePolicy clabtypes.LicensePolicy
	// OverwriteNode stores the interface used to overwrite methods defined
	// for DefaultNode, so that particular nodes can provide custom implementations.
	OverwriteNode NodeOverwrites
	// List of link endpoints that are connected to the node.
	Endpoints []clablinks.Endpoint
	// Interface aliasing-related variables
	InterfaceRegexp       *regexp.Regexp
	InterfaceMappedPrefix string
	InterfaceOffset       int
	InterfaceHelp         string
	FirstDataIfIndex      int
	// State of the node
	state      clabnodesstate.NodeState
	statemutex sync.RWMutex
	StopSignal string
}

// NewDefaultNode initializes the DefaultNode structure and receives a NodeOverwrites interface
// which is implemented by the node struct of a particular kind.
// This allows DefaultNode to access fields of the specific node struct in the methods defined for
// DefaultNode.
func NewDefaultNode(n NodeOverwrites) *DefaultNode {
	dn := &DefaultNode{
		HostRequirements: clabtypes.NewHostRequirements(),
		OverwriteNode:    n,
		LicensePolicy:    clabtypes.LicensePolicyNone,
		SSHConfig:        clabtypes.NewSSHConfig(),
	}

	dn.InterfaceMappedPrefix = "eth"
	dn.InterfaceOffset = 0
	dn.FirstDataIfIndex = 0

	return dn
}

func (d *DefaultNode) WithMgmtNet(mgmt *clabtypes.MgmtNet)                   { d.Mgmt = mgmt }
func (d *DefaultNode) WithRuntime(r clabruntime.ContainerRuntime)            { d.Runtime = r }
func (d *DefaultNode) GetRuntime() clabruntime.ContainerRuntime              { return d.Runtime }
func (d *DefaultNode) Config() *clabtypes.NodeConfig                         { return d.Cfg }
func (*DefaultNode) PostDeploy(_ context.Context, _ *PostDeployParams) error { return nil }

// PreDeploy is a common method for all nodes that is called before the node is deployed.
func (d *DefaultNode) PreDeploy(_ context.Context, params *PreDeployParams) error {
	_, err := d.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return nil
}

func (d *DefaultNode) SaveConfig(_ context.Context) (*SaveConfigResult, error) {
	// nodes should have the save method defined on their respective structs.
	// By default SaveConfig is a noop.
	log.Debugf("Save operation is currently not supported for %q node kind", d.Cfg.Kind)
	return nil, nil
}

// CheckDeploymentConditions wraps individual functions that check if a node
// satisfies deployment requirements.
func (d *DefaultNode) CheckDeploymentConditions(ctx context.Context) error {
	err := d.OverwriteNode.VerifyHostRequirements()
	if err != nil {
		return err
	}

	err = d.OverwriteNode.VerifyStartupConfig(d.Cfg.LabDir)
	if err != nil {
		return err
	}

	err = d.OverwriteNode.CheckInterfaceName()
	if err != nil {
		return err
	}

	err = d.OverwriteNode.VerifyLicenseFileExists(ctx)
	if err != nil {
		return err
	}

	err = d.OverwriteNode.VerifyContainerName()
	if err != nil {
		return err
	}

	return nil
}

func (d *DefaultNode) PullImage(ctx context.Context) error {
	for imageKey, imageName := range d.OverwriteNode.GetImages(ctx) {
		if imageName == "" {
			return fmt.Errorf(
				"missing required %q attribute for node %q",
				imageKey,
				d.Cfg.ShortName,
			)
		}
		err := d.Runtime.PullImage(ctx, imageName, d.Config().ImagePullPolicy)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DefaultNode) VerifyHostRequirements() error {
	return d.HostRequirements.Verify(d.Cfg.Kind, d.Cfg.ShortName)
}

func (d *DefaultNode) Deploy(ctx context.Context, _ *DeployParams) error {
	// Set the "CLAB_INTFS" variable to the number of interfaces (endpoints) a node has.
	// This env var does not count in the eth0 interface that is automatically created by the
	// container runtime. This env var is used by some containers (e.g. vrnetlab systems) to
	// postpone the startup until all interfaces
	// have been added to the container namespace.
	d.Config().Env[clabconstants.ClabEnvIntfs] = strconv.Itoa(len(d.GetEndpoints()))

	// create the container
	cID, err := d.Runtime.CreateContainer(ctx, d.Cfg)
	if err != nil {
		return err
	}

	// start the container
	_, err = d.Runtime.StartContainer(ctx, cID, d)
	if err != nil {
		return err
	}

	// Update the nodes state
	d.SetState(clabnodesstate.Deployed)

	return nil
}

// GetNSPath retrieves the nodes nspath.
func (d *DefaultNode) GetNSPath(ctx context.Context) (string, error) {
	var err error
	nsp := ""

	if d.Cfg.IsRootNamespaceBased {
		netns, err := ns.GetCurrentNS()
		if err != nil {
			return "", err
		}
		nsp = netns.Path()
	}
	if nsp == "" {
		nsp, err = d.Runtime.GetNSPath(ctx, d.Cfg.LongName)
		if err != nil {
			log.Errorf("Unable to determine NetNS Path for node %s: %v", d.Cfg.ShortName, err)
			return "", err
		}
	}

	return nsp, err
}

func (d *DefaultNode) Delete(ctx context.Context) error {
	for _, e := range d.Endpoints {
		err := e.GetLink().Remove(ctx)
		if err != nil {
			return err
		}
	}
	return d.Runtime.DeleteContainer(ctx, d.OverwriteNode.GetContainerName())
}

func (d *DefaultNode) GetImages(_ context.Context) map[string]string {
	return map[string]string{
		ImageKey: d.Cfg.Image,
	}
}

func (d *DefaultNode) GetContainers(ctx context.Context) ([]clabruntime.GenericContainer, error) {
	cnts, err := d.Runtime.ListContainers(ctx, []*clabtypes.GenericFilter{
		{
			FilterType: "name",
			Match:      d.OverwriteNode.GetContainerName(),
		},
	})
	if err != nil {
		return nil, err
	}
	// check that we retrieved some container information
	// otherwise throw ErrContainersNotFound error
	if len(cnts) == 0 {
		return nil, fmt.Errorf("Node: %s. %w", d.GetContainerName(), ErrContainersNotFound)
	}

	return cnts, err
}

func (d *DefaultNode) RunExecFromConfig(ctx context.Context, ec *clabexec.ExecCollection) error {
	for _, e := range d.Config().Exec {
		exec, err := clabexec.NewExecCmdFromString(e)
		if err != nil {
			log.Warnf("Failed to parse the command string: %s, %v", e, err)
		}

		res, err := d.OverwriteNode.RunExec(ctx, exec)
		if err != nil {
			// kinds which do not support exec functionality are skipped
			continue
		}

		ec.Add(d.GetShortName(), res)
	}
	return nil
}

func (d *DefaultNode) UpdateConfigWithRuntimeInfo(ctx context.Context) error {
	cnts, err := d.OverwriteNode.GetContainers(ctx)
	if err != nil {
		return err
	}

	if len(cnts) == 0 {
		return fmt.Errorf("no container runtime information retrieved")
	}

	// network settings of a first container only
	netSettings := cnts[0].NetworkSettings

	d.Cfg.MgmtIPv4Address = netSettings.IPv4addr
	d.Cfg.MgmtIPv4PrefixLength = netSettings.IPv4pLen
	d.Cfg.MgmtIPv6Address = netSettings.IPv6addr
	d.Cfg.MgmtIPv6PrefixLength = netSettings.IPv6pLen
	d.Cfg.MgmtIPv4Gateway = netSettings.IPv4Gw
	d.Cfg.MgmtIPv6Gateway = netSettings.IPv6Gw

	d.Cfg.ContainerID = cnts[0].ID

	d.Cfg.ResultingPortBindings = cnts[0].Ports

	return nil
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (d *DefaultNode) DeleteNetnsSymlink() error {
	log.Debugf("Deleting %s network namespace", d.OverwriteNode.GetContainerName())
	return clabutils.DeleteNetnsSymlink(d.OverwriteNode.GetContainerName())
}

func (d *DefaultNode) CheckInterfaceOverlap() error {
	var seenEps []clablinks.Endpoint

	for _, ep := range d.Endpoints {
		ifName := ep.GetIfaceName()
		for _, seenEp := range seenEps {
			if ifName == seenEp.GetIfaceName() {
				return fmt.Errorf("%q and %q have overlapping interface names", ep, seenEp)
			}
		}
		seenEps = append(seenEps, ep)
	}

	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file is in the
// expected range of name values.
// A no-op for the default node, specific nodes should implement this method.
func (d *DefaultNode) CheckInterfaceName() error {
	return d.CheckInterfaceOverlap()
}

// CalculateInterfaceIndex parses the supplied interface name with the InterfaceRegexp.
// Using the the port offset, it calculates the mapped interface index based on the InterfaceOffset
// and the first data interface index.
func (d *DefaultNode) CalculateInterfaceIndex(ifName string) (int, error) {
	captureGroups, err := clabutils.GetRegexpCaptureGroups(d.InterfaceRegexp, ifName)
	if err != nil {
		return 0, err
	}

	if parsedIndex, found := captureGroups["port"]; found {
		parsedIndexInt, err := strconv.Atoi(parsedIndex)
		if err != nil {
			return 0, fmt.Errorf(
				"%q parsed index %q could not be cast to an integer",
				ifName,
				parsedIndex,
			)
		}
		calculatedIndex := parsedIndexInt - d.InterfaceOffset + d.FirstDataIfIndex
		return calculatedIndex, nil
	} else {
		return 0, fmt.Errorf("%q does not have extracted interface index with regexp %q, 'port' capture group missing?", ifName, d.InterfaceRegexp)
	}
}

// GetMappedInterfaceName returns with a mapped interface name based on the mapped interface prefix
// and calculated mapped interface index.
func (d *DefaultNode) GetMappedInterfaceName(ifName string) (string, error) {
	ifIndex, err := d.OverwriteNode.CalculateInterfaceIndex(ifName)
	if err != nil {
		return "", err
	}
	if ifIndex < d.FirstDataIfIndex {
		return "", fmt.Errorf(
			"extracted interface index for %q is out of bounds: %d ! >= %d",
			ifName,
			ifIndex,
			d.FirstDataIfIndex,
		)
	}
	mappedIfName := fmt.Sprintf("%s%d", d.InterfaceMappedPrefix, ifIndex)

	return mappedIfName, nil
}

// VerifyStartupConfig verifies that startup config files exists on disks.
func (d *DefaultNode) VerifyStartupConfig(topoDir string) error {
	cfg := d.Config().StartupConfig
	if cfg == "" {
		return nil
	}

	rcfg := clabutils.ResolvePath(cfg, topoDir)
	if !clabutils.FileExists(rcfg) {
		return fmt.Errorf("node %q startup-config file not found by the path %s",
			d.OverwriteNode.GetContainerName(), rcfg)
	}

	return nil
}

// GenerateConfig generates configuration for the nodes
// out of the template `t` based on the node configuration and saves the result to dst.
// If the config file is already present in the node dir we do not regenerate the config unless
// EnforceStartupConfig is explicitly set to true and startup-config points to a file this will
// persist the changes that users make to a running config when booted from some startup config.
func (d *DefaultNode) GenerateConfig(dst, t string) error {
	// Check for incompatible options
	if d.Cfg.EnforceStartupConfig && d.Cfg.SuppressStartupConfig {
		return ErrIncompatibleOptions
	}
	if d.Cfg.EnforceStartupConfig && d.Cfg.StartupConfig == "" {
		return ErrNoStartupConfig
	}

	if d.Cfg.SuppressStartupConfig {
		log.Info("Startup config generation suppressed", "node", d.Cfg.ShortName)
		return nil
	}

	if !d.Cfg.EnforceStartupConfig && clabutils.FileExists(dst) {
		log.Debug("Existing config found", "node", d.Cfg.ShortName, "path", dst)
		return nil
	} else {
		log.Debug("Generating config", "node", d.Cfg.ShortName, "file", d.Cfg.StartupConfig)

		cfgBuf, err := clabutils.SubstituteEnvsAndTemplate(strings.NewReader(t), d.Cfg)
		if err != nil {
			return err
		}
		log.Debug("Generated config", "node", d.Cfg.ShortName, "content", cfgBuf.String())

		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.Write(cfgBuf.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// NodeOverwrites is an interface that every node implements.
// It is used to enable DefaultNode to get access to the particular node structs
// and is provided as an argument of the NewDefaultNode function.
// The methods defined for this interfaces are the methods that particular node has a custom
// implementation of.
type NodeOverwrites interface {
	VerifyStartupConfig(topoDir string) error
	CheckInterfaceName() error
	CalculateInterfaceIndex(ifName string) (int, error)
	GetMappedInterfaceName(ifName string) (string, error)
	VerifyHostRequirements() error
	PullImage(ctx context.Context) error
	GetImages(ctx context.Context) map[string]string
	GetContainers(ctx context.Context) ([]clabruntime.GenericContainer, error)
	GetContainerName() string
	VerifyContainerName() error
	VerifyLicenseFileExists(context.Context) error
	RunExec(context.Context, *clabexec.ExecCmd) (*clabexec.ExecResult, error)
	GetNSPath(ctx context.Context) (string, error)
}

// LoadStartupConfigFileVr templates a startup-config using the file specified for VM-based nodes in
// the topo
// and puts the resulting config file by the LabDir/configDirName/startupCfgFName path.
func LoadStartupConfigFileVr(node Node, configDirName, startupCfgFName string) error {
	nodeCfg := node.Config()
	// create config directory that will be bind mounted to vrnetlab container at / path
	clabutils.CreateDirectory(path.Join(nodeCfg.LabDir, configDirName),
		clabconstants.PermissionsOpen)

	if nodeCfg.StartupConfig != "" {
		// dstCfg is a path to a file on the clab host that will have rendered configuration
		dstCfg := filepath.Join(nodeCfg.LabDir, configDirName, startupCfgFName)

		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}

		cfgTemplate := string(c)

		err = node.GenerateConfig(dstCfg, cfgTemplate)
		if err != nil {
			log.Errorf("node=%s, failed to generate config: %v", nodeCfg.ShortName, err)
		}
	}
	return nil
}

// GetContainerName returns the name used by the runtime to identify the container
// e.g. ext-container nodes use the name as defined in the topo file, while most other containers
// use long (prefixed) name.
func (d *DefaultNode) GetContainerName() string {
	return d.Cfg.LongName
}

func (d *DefaultNode) VerifyContainerName() error {
	containerName := d.GetContainerName()
	if !containerNamePatternRe.MatchString(containerName) {
		return fmt.Errorf("Node name contains invalid characters: %s", containerName)
	}

	if len(containerName) > maxNameLength {
		log.Warn(
			"Node name will not resolve via DNS",
			"name",
			containerName,
			"reason",
			fmt.Sprintf("name exceeds %d characters", maxNameLength),
		)
	}

	if strings.ContainsAny(containerName, dnsIncompatibleChars) {
		log.Warn(
			"Node name will not resolve via DNS",
			"name",
			containerName,
			"reason",
			"name contains invalid characters such as '.' and/or '_'",
		)
	}

	return nil
}

// RunExec executes a single command for a node.
func (d *DefaultNode) RunExec(
	ctx context.Context,
	execCmd *clabexec.ExecCmd,
) (*clabexec.ExecResult, error) {
	execResult, err := d.GetRuntime().Exec(ctx, d.OverwriteNode.GetContainerName(), execCmd)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v",
			d.OverwriteNode.GetContainerName(), execCmd.GetCmdString(), err)
		return nil, err
	}
	return execResult, nil
}

// RunExecNotWait executes a command for a node, and doesn't block waiting for the output.
// Should be overridden if the nodes implementation differs.
func (d *DefaultNode) RunExecNotWait(ctx context.Context, execCmd *clabexec.ExecCmd) error {
	err := d.GetRuntime().ExecNotWait(ctx, d.OverwriteNode.GetContainerName(), execCmd)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v",
			d.OverwriteNode.GetContainerName(), execCmd.GetCmdString(), err)
		return err
	}
	return nil
}

// VerifyLicenseFileExists checks if a license file with a provided path exists.
func (d *DefaultNode) VerifyLicenseFileExists(_ context.Context) error {
	if d.Config().License == "" {
		switch d.LicensePolicy {
		// if a license is required by the kind but not provided
		case clabtypes.LicensePolicyRequired:
			return fmt.Errorf(
				"node %s of kind %s requires a license. Provide one via 'license' knob in the topology file",
				d.Config().ShortName,
				d.Cfg.Kind,
			)
		case clabtypes.LicensePolicyWarn:
			// just warn when no license is provided
			log.Warnf(
				"node %s of kind %s requires a license. Make sure to provide it in some way (e.g. license knob or baked into image)",
				d.Config().ShortName,
				d.Cfg.Kind,
			)
			return nil
		case clabtypes.LicensePolicyNone:
			// license is not required
			return nil
		default:
			return fmt.Errorf("unknown license policy value %s for node %s kind %s",
				d.LicensePolicy, d.Config().ShortName, d.Cfg.Kind)
		}
	}
	// if license is provided check path exists
	rlic := clabutils.ResolvePath(d.Config().License, d.Cfg.LabDir)
	if !clabutils.FileExists(rlic) {
		return fmt.Errorf(
			"license file for node %q is not found by the path %s",
			d.Config().ShortName,
			rlic,
		)
	}

	return nil
}

// LoadOrGenerateCertificate loads a certificate using a certificate storage provider
// provided in certInfra or generates a new one if it does not exist.
func (d *DefaultNode) LoadOrGenerateCertificate(
	certInfra *clabcert.Cert,
	topoName string,
) (nodeCert *clabcert.Certificate, err error) {
	// early return if certificate generation is not required
	if d.Cfg.Certificate == nil || !*d.Cfg.Certificate.Issue {
		return nil, nil
	}

	nodeConfig := d.Cfg

	// try loading existing certificates from disk and generate new ones if they do not exist
	nodeCert, err = certInfra.LoadNodeCert(nodeConfig.ShortName)
	if err != nil {
		log.Debugf("creating node certificate for %s", nodeConfig.ShortName)

		hosts := []string{
			nodeConfig.ShortName,
			nodeConfig.LongName,
			nodeConfig.ShortName + "." + topoName + ".io",
		}
		// add the SANs provided via config
		hosts = append(hosts, nodeConfig.Certificate.SANs...)

		// add mgmt IPs as SANs to CSR
		for _, ip := range []string{d.Cfg.MgmtIPv4Address, d.Cfg.MgmtIPv6Address} {
			if ip != "" {
				hosts = append(hosts, ip)
			}
		}

		certInput := &clabcert.NodeCSRInput{
			CommonName:   nodeConfig.ShortName + "." + topoName + ".io",
			Hosts:        hosts,
			Organization: "containerlab",
			Country:      "US",
			KeySize:      d.Cfg.Certificate.KeySize,
			Expiry:       d.Cfg.Certificate.ValidityDuration,
		}
		// Generate the cert for the node
		nodeCert, err = certInfra.GenerateAndSignNodeCert(certInput)
		if err != nil {
			return nil, err
		}

		// persist the cert via certStorage
		err = certInfra.StoreNodeCert(nodeConfig.ShortName, nodeCert)
		if err != nil {
			return nil, err
		}
	}

	return nodeCert, nil
}

func (d *DefaultNode) GetHostsEntries(ctx context.Context) (clabtypes.HostEntries, error) {
	containers, err := d.OverwriteNode.GetContainers(ctx)
	if err != nil {
		return nil, err
	}

	result := clabtypes.HostEntries{}

	for idx := range containers {
		if len(containers[idx].Names) == 0 {
			continue
		}
		if containers[idx].NetworkSettings.IPv4addr != "" {
			result = append(result, clabtypes.NewHostEntry(
				containers[idx].NetworkSettings.IPv4addr,
				containers[idx].Names[0],
				clabtypes.IpVersionV4,
			).SetDescription(fmt.Sprintf("Kind: %s", d.Cfg.Kind)).SetContainerID(containers[idx].ID))
		}
		if containers[idx].NetworkSettings.IPv6addr != "" {
			result = append(result, clabtypes.NewHostEntry(
				containers[idx].NetworkSettings.IPv6addr,
				containers[idx].Names[0],
				clabtypes.IpVersionV6,
			).SetDescription(fmt.Sprintf("Kind: %s", d.Cfg.Kind)).SetContainerID(containers[idx].ID))
		}
	}
	return result, nil
}

func (d *DefaultNode) AddLinkToContainer(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	// retrieve nodes nspath
	nsp, err := d.OverwriteNode.GetNSPath(ctx)
	if err != nil {
		return err
	}
	// retrieve the namespace handle
	netns, err := ns.GetNS(nsp)
	if err != nil {
		return err
	}
	// move veth endpoint to namespace
	if err := netlink.LinkSetNsFd(link, int(netns.Fd())); err != nil {
		return err
	}
	// execute the given function
	err = netns.Do(f)
	if err != nil {
		return err
	}
	return nil
}

// ExecFunction executes the given function in the nodes network namespace.
func (d *DefaultNode) ExecFunction(ctx context.Context, f func(ns.NetNS) error) error {
	// retrieve nodes nspath
	nspath, err := d.OverwriteNode.GetNSPath(ctx)
	if err != nil {
		return err
	}

	if d.Cfg.IsRootNamespaceBased {
		nshandle, err := ns.GetCurrentNS()
		if err != nil {
			return err
		}
		nspath = nshandle.Path()
	}

	if nspath == "" {
		return fmt.Errorf("nspath is not set for node %q", d.GetShortName())
	}

	// retrieve the namespace handle
	netns, err := ns.GetNS(nspath)
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

// AddEndpoint maps the endpoint name to before adding it to the node endpoints if it matches the
// interface alias regexp. Returns an error if the mapping goes wrong.
func (d *DefaultNode) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	if d.InterfaceRegexp != nil && d.InterfaceRegexp.MatchString(endpointName) {
		mappedName, err := d.OverwriteNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf(
				"%q interface name %q could not be mapped: %w",
				d.Cfg.ShortName,
				e.GetIfaceName(),
				err,
			)
		}
		log.Debugf(
			"Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)",
			endpointName,
			mappedName,
		)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}
	d.Endpoints = append(d.Endpoints, e)

	return nil
}

func (d *DefaultNode) GetEndpoints() []clablinks.Endpoint {
	return d.Endpoints
}

// GetLinkEndpointType returns a veth link endpoint type for default nodes.
// The LinkEndpointTypeVeth indicates a veth endpoint which doesn't require special handling.
func (*DefaultNode) GetLinkEndpointType() clablinks.LinkEndpointType {
	return clablinks.LinkEndpointTypeVeth
}

func (d *DefaultNode) GetShortName() string {
	return d.Cfg.ShortName
}

// DeployEndpoints deploys endpoints associated with the node.
// The deployment of endpoints is done by deploying a link with the endpoint triggering it.
func (d *DefaultNode) DeployEndpoints(ctx context.Context) error {
	for _, ep := range d.Endpoints {
		err := ep.Deploy(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DefaultNode) GetState() clabnodesstate.NodeState {
	d.statemutex.RLock()
	defer d.statemutex.RUnlock()
	return d.state
}

func (d *DefaultNode) SetState(s clabnodesstate.NodeState) {
	d.statemutex.Lock()
	defer d.statemutex.Unlock()
	d.state = s
}

func (d *DefaultNode) GetSSHConfig() *clabtypes.SSHConfig {
	return d.SSHConfig
}

func (d *DefaultNode) GetContainerStatus(ctx context.Context) clabruntime.ContainerStatus {
	return d.Runtime.GetContainerStatus(ctx, d.GetContainerName())
}

func (d *DefaultNode) IsHealthy(ctx context.Context) (bool, error) {
	return d.Runtime.IsHealthy(ctx, d.GetContainerName())
}

// ---------------------------------------------------------------------------
// Lifecycle: Stop / Start
// ---------------------------------------------------------------------------

const vrnetlabVersionLabel = "vrnetlab-version"

var imageTagRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Stop parks dataplane interfaces into a dedicated network namespace and stops the container.
func (d *DefaultNode) Stop(ctx context.Context) error {
	cfg := d.Config()

	parkName := clabutils.ParkingNetnsName(cfg.LongName)
	parkPath, err := ensureNamedNetns(parkName)
	if err != nil {
		return fmt.Errorf("node %q failed creating parking netns: %w", cfg.ShortName, err)
	}
	parkingNode := clablinks.NewGenericLinkNode(parkName, parkPath)

	nodeNSPath, err := d.GetNSPath(ctx)
	if err != nil {
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	// Move dataplane interfaces into the parking netns while the container netns is still alive.
	moved, err := moveEndpoints(
		d.GetEndpoints(),
		func(ep clablinks.Endpoint) error {
			return ep.MoveTo(ctx, parkingNode, preMoveSetDownOptions())
		},
	)
	if err != nil {
		// Roll back any endpoints already moved to the parking namespace.
		if len(moved) > 0 {
			if rbErr := rollbackEndpoints(moved, func(ep clablinks.Endpoint) error {
				return ep.MoveTo(ctx, d, nil)
			}); rbErr != nil {
				return fmt.Errorf(
					"node %q failed parking interfaces: %w (rollback failed: %v)",
					cfg.ShortName,
					err,
					rbErr,
				)
			}
			_ = setEndpointsUp(ctx, moved)
		}
		return fmt.Errorf("node %q failed parking interfaces: %w", cfg.ShortName, err)
	}

	// Repoint /run/netns/<containerName> to the parking netns so that inspect/destroy keep working.
	if err := clabutils.LinkContainerNS(parkPath, cfg.LongName); err != nil {
		return fmt.Errorf("node %q failed linking parking netns: %w", cfg.ShortName, err)
	}

	preStopCleanup(ctx, d)

	if err := d.Runtime.StopContainer(ctx, cfg.LongName, d.StopSignal); err != nil {
		// Docker/podman may return an error while the container is already stopped (timeout, API hiccup).
		// Treat this as success if the desired state is reached.
		status := d.Runtime.GetContainerStatus(ctx, cfg.LongName)
		if status == clabruntime.Stopped {
			log.Warnf("node %q stop returned error but container is stopped: %v", cfg.ShortName, err)
			return nil
		}

		// Roll back to a running node with interfaces restored.
		if linkErr := clabutils.LinkContainerNS(nodeNSPath, cfg.LongName); linkErr != nil {
			log.Warnf("node %q failed restoring /run/netns symlink after stop error: %v", cfg.ShortName, linkErr)
		}
		if rbErr := rollbackEndpoints(moved, func(ep clablinks.Endpoint) error {
			return ep.MoveTo(ctx, d, nil)
		}); rbErr != nil {
			return fmt.Errorf(
				"node %q failed stopping container: %w (rollback failed restoring interfaces: %v)",
				cfg.ShortName,
				err,
				rbErr,
			)
		}
		_ = setEndpointsUp(ctx, moved)

		return fmt.Errorf("node %q failed stopping container: %w", cfg.ShortName, err)
	}

	return nil
}

// Start restarts a stopped container and restores its parked dataplane interfaces.
func (d *DefaultNode) Start(ctx context.Context) error {
	cfg := d.Config()

	parkName := clabutils.ParkingNetnsName(cfg.LongName)
	parkPath := filepath.Join("/run/netns", parkName)
	if !clabutils.FileOrDirExists(parkPath) {
		return fmt.Errorf(
			"node %q has no parking netns %q; seamless start requires stopping via containerlab",
			cfg.ShortName,
			parkName,
		)
	}
	parkingNode := clablinks.NewGenericLinkNode(parkName, parkPath)

	if _, err := d.Runtime.StartContainer(ctx, cfg.LongName, d); err != nil {
		return fmt.Errorf("node %q failed starting container: %w", cfg.ShortName, err)
	}

	if _, err := d.GetNSPath(ctx); err != nil {
		// Try to keep destroy/inspect operational by repointing back to the parking netns.
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = d.Runtime.StopContainer(ctx, cfg.LongName, d.StopSignal)
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	// Reparent endpoints to the parking node so MoveTo knows where to find them.
	// Stop and Start are separate process invocations, so the in-memory node
	// reference from Stop does not survive; we restore it here.
	for _, ep := range d.GetEndpoints() {
		ep.SetNode(parkingNode)
	}

	// Move interfaces back into the container netns.
	moved, err := moveEndpoints(d.GetEndpoints(), func(ep clablinks.Endpoint) error {
		return ep.MoveTo(ctx, d, nil)
	})
	if err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_ = rollbackEndpoints(moved, func(ep clablinks.Endpoint) error {
			return ep.MoveTo(ctx, parkingNode, preMoveSetDownOptions())
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = d.Runtime.StopContainer(ctx, cfg.LongName, d.StopSignal)
		return fmt.Errorf("node %q failed restoring interfaces: %w", cfg.ShortName, err)
	}

	// Bring restored interfaces up.
	if err := setEndpointsUp(ctx, moved); err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_ = rollbackEndpoints(moved, func(ep clablinks.Endpoint) error {
			return ep.MoveTo(ctx, parkingNode, preMoveSetDownOptions())
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = d.Runtime.StopContainer(ctx, cfg.LongName, d.StopSignal)
		return fmt.Errorf("node %q failed enabling interfaces: %w", cfg.ShortName, err)
	}

	// Re-run topology exec commands on lifecycle start to restore node-local interface config
	// (for example IP addresses added during deploy exec phase).
	execCollection := clabexec.NewExecCollection()
	if err := d.RunExecFromConfig(ctx, execCollection); err != nil {
		log.Errorf("failed to run exec commands for node %q on lifecycle start: %v", cfg.ShortName, err)
	}
	execCollection.Log()

	return nil
}

// ---------------------------------------------------------------------------
// Lifecycle helpers (unexported)
// ---------------------------------------------------------------------------

func ensureNamedNetns(name string) (string, error) {
	nspath := filepath.Join("/run/netns", name)
	if clabutils.FileOrDirExists(nspath) {
		return nspath, nil
	}

	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	origNS, err := netns.Get()
	if err != nil {
		return "", err
	}
	defer origNS.Close()
	defer func() {
		_ = netns.Set(origNS)
	}()

	newNS, err := netns.NewNamed(name)
	if err != nil {
		if os.IsExist(err) && clabutils.FileOrDirExists(nspath) {
			return nspath, nil
		}
		return "", err
	}
	newNS.Close()

	return nspath, nil
}

func preMoveSetDownOptions() *clablinks.MoveOptions {
	return &clablinks.MoveOptions{
		PreMove: netlink.LinkSetDown,
	}
}

func moveEndpoints(
	endpoints []clablinks.Endpoint,
	move func(clablinks.Endpoint) error,
) ([]clablinks.Endpoint, error) {
	moved := make([]clablinks.Endpoint, 0, len(endpoints))

	for _, ep := range endpoints {
		if err := move(ep); err != nil {
			return moved, err
		}
		moved = append(moved, ep)
	}

	return moved, nil
}

func rollbackEndpoints(
	moved []clablinks.Endpoint,
	move func(clablinks.Endpoint) error,
) error {
	for i := len(moved) - 1; i >= 0; i-- {
		if err := move(moved[i]); err != nil {
			return err
		}
	}

	return nil
}

func setEndpointsUp(ctx context.Context, endpoints []clablinks.Endpoint) error {
	for _, ep := range endpoints {
		if err := ep.SetUp(ctx); err != nil {
			return err
		}
	}

	return nil
}

func preStopCleanup(ctx context.Context, d *DefaultNode) {
	if isVrnetlabNode(ctx, d) {
		preStopPrepareVrnetlabQcowAlias(ctx, d)
	}

	preStopCleanupNamedNetns(ctx, d)
}

func preStopCleanupNamedNetns(ctx context.Context, d *DefaultNode) {
	// Best-effort cleanup for containers that create named network namespaces under /run/netns.
	// We lazily unmount active nsfs mounts and remove stale entries to avoid namespace artifacts
	// breaking subsequent starts.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := `if [ -d /run/netns ]; then ` +
		`awk '$5 ~ "^/run/netns/" {print $5}' /proc/self/mountinfo 2>/dev/null | ` +
		`while IFS= read -r mp; do ` +
		`umount -l "$mp" 2>/dev/null || true; ` +
		`rm -f "$mp" 2>/dev/null || true; ` +
		`done; ` +
		`for f in /run/netns/*; do [ -e "$f" ] || break; rm -f "$f" 2>/dev/null || true; done; ` +
		`fi`

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	if res, err := d.RunExec(ctx, execCmd); err != nil {
		log.Debugf(
			"node %q generic pre-stop named-netns cleanup skipped/failed: %v",
			d.Config().ShortName,
			err,
		)
	} else if res != nil && res.ReturnCode != 0 {
		log.Debugf(
			"node %q generic pre-stop named-netns cleanup returned code %d (stderr: %s)",
			d.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func isVrnetlabNode(ctx context.Context, d *DefaultNode) bool {
	containers, err := d.GetContainers(ctx)
	if err != nil {
		log.Debugf(
			"node %q vrnetlab detection skipped: failed to get container metadata: %v",
			d.Config().ShortName,
			err,
		)
		return false
	}

	for _, container := range containers {
		if _, ok := container.Labels[vrnetlabVersionLabel]; ok {
			return true
		}
	}

	return false
}

func preStopPrepareVrnetlabQcowAlias(ctx context.Context, d *DefaultNode) {
	aliasName, ok := vrnetlabQcowAliasName(d.Config().Image)
	if !ok {
		log.Debugf(
			"node %q pre-stop vrnetlab qcow alias skipped: unable to infer tag from image %q",
			d.Config().ShortName,
			d.Config().Image,
		)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Some vrnetlab nodes rename the original versioned qcow image after first boot and fail on
	// subsequent starts when they try to rediscover a versioned qcow filename. If there is exactly
	// one non-overlay qcow file in / and our alias is absent, create a hardlink alias based on the
	// image tag.
	cmd := fmt.Sprintf(
		`alias="/%s"; `+
			`[ -e "$alias" ] && exit 0; `+
			`src=""; `+
			`if [ -f /sros.qcow2 ] && [ "/sros.qcow2" != "$alias" ]; then `+
			`src="/sros.qcow2"; `+
			`else `+
			`set -- /*.qcow2; `+
			`if [ "$1" != "/*.qcow2" ]; then `+
			`for f in "$@"; do `+
			`[ "$f" = "$alias" ] && continue; `+
			`base="${f##*/}"; `+
			`case "$base" in *overlay*.qcow2) continue ;; esac; `+
			`if [ -n "$src" ]; then src=""; break; fi; `+
			`src="$f"; `+
			`done; `+
			`fi; `+
			`fi; `+
			`[ -n "$src" ] || exit 0; `+
			`ln "$src" "$alias"`,
		aliasName,
	)

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	res, err := d.RunExec(ctx, execCmd)
	if err != nil {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias preparation failed: %v",
			d.Config().ShortName,
			err,
		)
		return
	}

	if res != nil && res.ReturnCode != 0 {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias prep returned code %d (stderr: %s)",
			d.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func vrnetlabQcowAliasName(image string) (string, bool) {
	tag, ok := imageTag(image)
	if !ok {
		return "", false
	}

	return "clab-" + tag + ".qcow2", true
}

func imageTag(image string) (string, bool) {
	if at := strings.LastIndex(image, "@"); at != -1 {
		image = image[:at]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 || lastColon < lastSlash {
		return "", false
	}

	tag := image[lastColon+1:]
	if tag == "" {
		return "", false
	}

	if !imageTagRE.MatchString(tag) {
		return "", false
	}

	return tag, true
}

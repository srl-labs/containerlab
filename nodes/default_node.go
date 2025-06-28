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
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// DefaultNode implements the Node interface and is embedded to the structs of all other nodes.
// It has common fields and methods that every node should typically have. Nodes can override methods if needed.
type DefaultNode struct {
	Cfg              *types.NodeConfig
	Mgmt             *types.MgmtNet
	Runtime          runtime.ContainerRuntime
	HostRequirements *types.HostRequirements
	// SSHConfig is the SSH client configuration that a clab node requires.
	SSHConfig *types.SSHConfig
	// Indicates that the node should not start without no license file defined
	LicensePolicy types.LicensePolicy
	// OverwriteNode stores the interface used to overwrite methods defined
	// for DefaultNode, so that particular nodes can provide custom implementations.
	OverwriteNode NodeOverwrites
	// List of link endpoints that are connected to the node.
	Endpoints []links.Endpoint
	// Interface aliasing-related variables
	InterfaceRegexp       *regexp.Regexp
	InterfaceMappedPrefix string
	InterfaceOffset       int
	InterfaceHelp         string
	FirstDataIfIndex      int
	// State of the node
	state      state.NodeState
	statemutex sync.RWMutex
}

// NewDefaultNode initializes the DefaultNode structure and receives a NodeOverwrites interface
// which is implemented by the node struct of a particular kind.
// This allows DefaultNode to access fields of the specific node struct in the methods defined for DefaultNode.
func NewDefaultNode(n NodeOverwrites) *DefaultNode {
	dn := &DefaultNode{
		HostRequirements: types.NewHostRequirements(),
		OverwriteNode:    n,
		LicensePolicy:    types.LicensePolicyNone,
		SSHConfig:        types.NewSSHConfig(),
	}

	dn.InterfaceMappedPrefix = "eth"
	dn.InterfaceOffset = 0
	dn.FirstDataIfIndex = 0

	return dn
}

func (d *DefaultNode) WithMgmtNet(mgmt *types.MgmtNet)                       { d.Mgmt = mgmt }
func (d *DefaultNode) WithRuntime(r runtime.ContainerRuntime)                { d.Runtime = r }
func (d *DefaultNode) GetRuntime() runtime.ContainerRuntime                  { return d.Runtime }
func (d *DefaultNode) Config() *types.NodeConfig                             { return d.Cfg }
func (*DefaultNode) PostDeploy(_ context.Context, _ *PostDeployParams) error { return nil }

// PreDeploy is a common method for all nodes that is called before the node is deployed.
func (d *DefaultNode) PreDeploy(_ context.Context, params *PreDeployParams) error {
	_, err := d.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return nil
}

func (d *DefaultNode) SaveConfig(_ context.Context) error {
	// nodes should have the save method defined on their respective structs.
	// By default SaveConfig is a noop.
	log.Debugf("Save operation is currently not supported for %q node kind", d.Cfg.Kind)
	return nil
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

	err = d.OverwriteNode.PullImage(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (d *DefaultNode) PullImage(ctx context.Context) error {
	for imageKey, imageName := range d.OverwriteNode.GetImages(ctx) {
		if imageName == "" {
			return fmt.Errorf("missing required %q attribute for node %q", imageKey, d.Cfg.ShortName)
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
	// This env var does not count in the eth0 interface that is automatically created by the container runtime.
	// This env var is used by some containers (e.g. vrnetlab systems) to postpone the startup until all interfaces
	// have been added to the container namespace.
	d.Config().Env[types.CLAB_ENV_INTFS] = strconv.Itoa(len(d.GetEndpoints()))

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
	d.SetState(state.Deployed)

	return nil
}

// getNSPath retrieves the nodes nspath.
func (d *DefaultNode) getNSPath(ctx context.Context) (string, error) {
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

func (d *DefaultNode) GetContainers(ctx context.Context) ([]runtime.GenericContainer, error) {
	cnts, err := d.Runtime.ListContainers(ctx, []*types.GenericFilter{
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

func (d *DefaultNode) RunExecFromConfig(ctx context.Context, ec *exec.ExecCollection) error {
	for _, e := range d.Config().Exec {
		exec, err := exec.NewExecCmdFromString(e)
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
	return utils.DeleteNetnsSymlink(d.OverwriteNode.GetContainerName())
}

func (d *DefaultNode) CheckInterfaceOverlap() error {
	var seenEps []links.Endpoint

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

// CheckInterfaceName checks if a name of the interface referenced in the topology file is in the expected range of name values.
// A no-op for the default node, specific nodes should implement this method.
func (d *DefaultNode) CheckInterfaceName() error {
	return d.CheckInterfaceOverlap()
}

// CalculateInterfaceIndex parses the supplied interface name with the InterfaceRegexp.
// Using the the port offset, it calculates the mapped interface index based on the InterfaceOffset and the first data interface index.
func (d *DefaultNode) CalculateInterfaceIndex(ifName string) (int, error) {
	captureGroups, err := utils.GetRegexpCaptureGroups(d.InterfaceRegexp, ifName)
	if err != nil {
		return 0, err
	}

	if parsedIndex, found := captureGroups["port"]; found {
		parsedIndexInt, err := strconv.Atoi(parsedIndex)
		if err != nil {
			return 0, fmt.Errorf("%q parsed index %q could not be cast to an integer", ifName, parsedIndex)
		}
		calculatedIndex := parsedIndexInt - d.InterfaceOffset + d.FirstDataIfIndex
		return calculatedIndex, nil
	} else {
		return 0, fmt.Errorf("%q does not have extracted interface index with regexp %q, 'port' capture group missing?", ifName, d.InterfaceRegexp)
	}
}

// GetMappedInterfaceName returns with a mapped interface name based on the mapped interface prefix and calculated mapped interface index.
func (d *DefaultNode) GetMappedInterfaceName(ifName string) (string, error) {
	ifIndex, err := d.OverwriteNode.CalculateInterfaceIndex(ifName)
	if err != nil {
		return "", err
	}
	if !(ifIndex >= d.FirstDataIfIndex) {
		return "", fmt.Errorf("extracted interface index for %q is out of bounds: %d ! >= %d", ifName, ifIndex, d.FirstDataIfIndex)
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

	rcfg := utils.ResolvePath(cfg, topoDir)
	if !utils.FileExists(rcfg) {
		return fmt.Errorf("node %q startup-config file not found by the path %s",
			d.OverwriteNode.GetContainerName(), rcfg)
	}

	return nil
}

// GenerateConfig generates configuration for the nodes
// out of the template `t` based on the node configuration and saves the result to dst.
// If the config file is already present in the node dir
// we do not regenerate the config unless EnforceStartupConfig is explicitly set to true and startup-config points to a file
// this will persist the changes that users make to a running config when booted from some startup config
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

	if !d.Cfg.EnforceStartupConfig && utils.FileExists(dst) {
		log.Debug("Existing config found", "node", d.Cfg.ShortName, "path", dst)
		return nil
	} else {

		log.Debug("Generating config", "node", d.Cfg.ShortName, "file", d.Cfg.StartupConfig)

		cfgBuf, err := utils.SubstituteEnvsAndTemplate(strings.NewReader(t), d.Cfg)
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
	GetContainers(ctx context.Context) ([]runtime.GenericContainer, error)
	GetContainerName() string
	VerifyLicenseFileExists(context.Context) error
	RunExec(context.Context, *exec.ExecCmd) (*exec.ExecResult, error)
}

// LoadStartupConfigFileVr templates a startup-config using the file specified for VM-based nodes in the topo
// and puts the resulting config file by the LabDir/configDirName/startupCfgFName path.
func LoadStartupConfigFileVr(node Node, configDirName, startupCfgFName string) error {
	nodeCfg := node.Config()
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, configDirName), 0777)

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
// e.g. ext-container nodes use the name as defined in the topo file, while most other containers use long (prefixed) name.
func (d *DefaultNode) GetContainerName() string {
	return d.Cfg.LongName
}

// RunExec executes a single command for a node.
func (d *DefaultNode) RunExec(ctx context.Context, execCmd *exec.ExecCmd) (*exec.ExecResult, error) {
	execResult, err := d.GetRuntime().Exec(ctx, d.OverwriteNode.GetContainerName(), execCmd)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v",
			d.OverwriteNode.GetContainerName(), execCmd.GetCmdString(), err)
		return nil, err
	}
	return execResult, nil
}

// RunExecNotWait executes a command for a node, and doesn't block waiting for the output.
// Should be overriden if the nodes implementation differs.
func (d *DefaultNode) RunExecNotWait(ctx context.Context, execCmd *exec.ExecCmd) error {
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
		case types.LicensePolicyRequired:
			return fmt.Errorf("node %s of kind %s requires a license. Provide one via 'license' knob in the topology file", d.Config().ShortName, d.Cfg.Kind)
		case types.LicensePolicyWarn:
			// just warn when no license is provided
			log.Warnf("node %s of kind %s requires a license. Make sure to provide it in some way (e.g. license knob or baked into image)", d.Config().ShortName, d.Cfg.Kind)
			return nil
		case types.LicensePolicyNone:
			// license is not required
			return nil
		default:
			return fmt.Errorf("unknown license policy value %s for node %s kind %s",
				d.LicensePolicy, d.Config().ShortName, d.Cfg.Kind)
		}
	}
	// if license is provided check path exists
	rlic := utils.ResolvePath(d.Config().License, d.Cfg.LabDir)
	if !utils.FileExists(rlic) {
		return fmt.Errorf("license file for node %q is not found by the path %s", d.Config().ShortName, rlic)
	}

	return nil
}

// LoadOrGenerateCertificate loads a certificate using a certificate storage provider
// provided in certInfra or generates a new one if it does not exist.
func (d *DefaultNode) LoadOrGenerateCertificate(certInfra *cert.Cert, topoName string) (nodeCert *cert.Certificate, err error) {
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

		certInput := &cert.NodeCSRInput{
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

func (d *DefaultNode) AddLinkToContainer(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve nodes nspath
	nsp, err := d.getNSPath(ctx)
	if err != nil {
		return err
	}
	// retrieve the namespace handle
	netns, err := ns.GetNS(nsp)
	if err != nil {
		return err
	}
	// move veth endpoint to namespace
	if err = netlink.LinkSetNsFd(link, int(netns.Fd())); err != nil {
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
	nspath, err := d.getNSPath(ctx)
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

// AddEndpoint maps the endpoint name to before adding it to the node endpoints if it matches the interface alias regexp. Returns an error if the mapping goes wrong.
func (d *DefaultNode) AddEndpoint(e links.Endpoint) error {
	endpointName := e.GetIfaceName()
	if d.InterfaceRegexp != nil && d.InterfaceRegexp.MatchString(endpointName) {
		mappedName, err := d.OverwriteNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf("%q interface name %q could not be mapped: %w", d.Cfg.ShortName, e.GetIfaceName(), err)
		}
		log.Debugf("Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)", endpointName, mappedName)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}
	d.Endpoints = append(d.Endpoints, e)

	return nil
}

func (d *DefaultNode) GetEndpoints() []links.Endpoint {
	return d.Endpoints
}

// GetLinkEndpointType returns a veth link endpoint type for default nodes.
// The LinkEndpointTypeVeth indicates a veth endpoint which doesn't require special handling.
func (*DefaultNode) GetLinkEndpointType() links.LinkEndpointType {
	return links.LinkEndpointTypeVeth
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

func (d *DefaultNode) GetState() state.NodeState {
	d.statemutex.RLock()
	defer d.statemutex.RUnlock()
	return d.state
}

func (d *DefaultNode) SetState(s state.NodeState) {
	d.statemutex.Lock()
	defer d.statemutex.Unlock()
	d.state = s
}

func (d *DefaultNode) GetSSHConfig() *types.SSHConfig {
	return d.SSHConfig
}

func (d *DefaultNode) GetContainerStatus(ctx context.Context) runtime.ContainerStatus {
	return d.Runtime.GetContainerStatus(ctx, d.GetContainerName())
}

func (d *DefaultNode) IsHealthy(ctx context.Context) (bool, error) {
	return d.Runtime.IsHealthy(ctx, d.GetContainerName())
}

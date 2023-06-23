// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

// DefaultNode implements the Node interface and is embedded to the structs of all other nodes.
// It has common fields and methods that every node should typically have. Nodes can override methods if needed.
type DefaultNode struct {
	Cfg              *types.NodeConfig
	Mgmt             *types.MgmtNet
	Runtime          runtime.ContainerRuntime
	HostRequirements *types.HostRequirements
	// Indicates that the node should not start without no license file defined
	LicensePolicy types.LicensePolicy
	// OverwriteNode stores the interface used to overwrite methods defined
	// for DefaultNode, so that particular nodes can provide custom implementations.
	OverwriteNode NodeOverwrites
}

// NewDefaultNode initializes the DefaultNode structure and receives a NodeOverwrites interface
// which is implemented by the node struct of a particular kind.
// This allows DefaultNode to access fields of the specific node struct in the methods defined for DefaultNode.
func NewDefaultNode(n NodeOverwrites) *DefaultNode {
	dn := &DefaultNode{
		HostRequirements: types.NewHostRequirements(),
		OverwriteNode:    n,
		LicensePolicy:    types.LicensePolicyNone,
	}

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
	cID, err := d.Runtime.CreateContainer(ctx, d.Cfg)
	if err != nil {
		return err
	}
	_, err = d.Runtime.StartContainer(ctx, cID, d.Cfg)
	return err
}

func (d *DefaultNode) Delete(ctx context.Context) error {
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
			Match:      fmt.Sprintf("^%s$", d.OverwriteNode.GetContainerName()), // this regexp ensure we have an exact match for name
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

// CheckInterfaceName checks if a name of the interface referenced in the topology file is in the expected range of name values.
// A no-op for the default node, specific nodes should implement this method.
func (*DefaultNode) CheckInterfaceName() error {
	return nil
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
// out of the template based on the node configuration and saves the result to dst.
func (d *DefaultNode) GenerateConfig(dst, templ string) error {
	// If the config file is already present in the node dir
	// we do not regenerate the config unless EnforceStartupConfig is explicitly set to true and startup-config points to a file
	// this will persist the changes that users make to a running config when booted from some startup config
	if utils.FileExists(dst) && (d.Cfg.StartupConfig == "" || !d.Cfg.EnforceStartupConfig) {
		log.Infof("config file '%s' for node '%s' already exists and will not be generated/reset", dst, d.Cfg.ShortName)
		return nil
	} else if d.Cfg.EnforceStartupConfig {
		log.Infof("Startup config for '%s' node enforced: '%s'", d.Cfg.ShortName, dst)
	}

	log.Debugf("generating config for node %s from file %s", d.Cfg.ShortName, d.Cfg.StartupConfig)

	// gomplate overrides the built-in *slice* function. You can still use *coll.Slice*
	gfuncs := gomplate.CreateFuncs(context.Background(), new(data.Data))
	delete(gfuncs, "slice")
	tpl, err := template.New(filepath.Base(d.Cfg.StartupConfig)).Funcs(gfuncs).Parse(templ)
	if err != nil {
		return err
	}

	dstBytes := new(bytes.Buffer)

	err = tpl.Execute(dstBytes, d.Cfg)
	if err != nil {
		return err
	}
	log.Debugf("node '%s' generated config: %s", d.Cfg.ShortName, dstBytes.String())

	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = f.Write(dstBytes.Bytes())
	if err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

// NodeOverwrites is an interface that every node implementation implements.
// It is used to enable DefaultNode to get access to the particular node structs
// and is provided as an argument of the NewDefaultNode function.
// The methods defined for this interfaces are the methods that particular node has a custom
// implementation of.
type NodeOverwrites interface {
	VerifyStartupConfig(topoDir string) error
	CheckInterfaceName() error
	VerifyHostRequirements() error
	PullImage(ctx context.Context) error
	GetImages(ctx context.Context) map[string]string
	GetContainers(ctx context.Context) ([]runtime.GenericContainer, error)
	GetContainerName() string
	VerifyLicenseFileExists(context.Context) error
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
		return fmt.Errorf("license file of node %q not found by the path %s", d.Config().ShortName, rlic)
	}
	return nil
}

// LoadOrGenerateCertificate loads a certificate using a certificate storage provider
// provided in certInfra or generates a new one if it does not exist.
func (d *DefaultNode) LoadOrGenerateCertificate(certInfra *cert.Cert, topoName string) (nodeCert *cert.Certificate, err error) {
	// early return if certificate generation is not required
	if d.Cfg.Certificate == nil || !d.Cfg.Certificate.Issue {
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
		hosts = append(hosts, nodeConfig.SANs...)

		// collect cert details
		certInput := &cert.NodeCSRInput{
			CommonName:   nodeConfig.ShortName + "." + topoName + ".io",
			Hosts:        hosts,
			Organization: "containerlab",
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

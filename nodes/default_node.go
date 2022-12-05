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
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

type DefaultNode struct {
	Cfg              *types.NodeConfig
	Mgmt             *types.MgmtNet
	Runtime          runtime.ContainerRuntime
	HostRequirements types.HostRequirements
	OverwriteNode    NodeOverwrites
}

func NewDefaultNode(on NodeOverwrites) *DefaultNode {
	dn := &DefaultNode{
		HostRequirements: types.HostRequirements{},
		OverwriteNode:    on,
	}
	return dn
}

func (d *DefaultNode) PostDeploy(_ context.Context, _ map[string]Node) error {
	return nil
}
func (d *DefaultNode) WithMgmtNet(mgmt *types.MgmtNet)                   { d.Mgmt = mgmt }
func (d *DefaultNode) WithRuntime(r runtime.ContainerRuntime)            { d.Runtime = r }
func (d *DefaultNode) GetRuntime() runtime.ContainerRuntime              { return d.Runtime }
func (d *DefaultNode) Config() *types.NodeConfig                         { return d.Cfg }
func (d *DefaultNode) PreDeploy(_ context.Context, _, _, _ string) error { return nil }

func (d *DefaultNode) SaveConfig(_ context.Context) error {
	log.Debugf("Save operation is currently not supported for %q node kind", d.Cfg.Kind)
	return nil
}

func (d *DefaultNode) PreCheckDeploymentConditionsMeet(ctx context.Context) error {
	err := d.OverwriteNode.VerifyHostRequirements()
	if err != nil {
		return err
	}
	err = d.OverwriteNode.VerifyStartupConfig(d.Cfg.LabDir)
	if err != nil {
		return err
	}
	err = d.OverwriteNode.CheckInterfaceNamingConvention()
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
		err := d.Runtime.PullImageIfRequired(ctx, imageName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DefaultNode) VerifyHostRequirements() error {
	_, err := d.HostRequirements.IsValid()
	return err
}

func (d *DefaultNode) Deploy(ctx context.Context) error {
	cID, err := d.Runtime.CreateContainer(ctx, d.Cfg)
	if err != nil {
		return err
	}
	_, err = d.Runtime.StartContainer(ctx, cID, d.Cfg)
	return err
}

func (d *DefaultNode) Delete(ctx context.Context) error {
	return d.Runtime.DeleteContainer(ctx, d.Cfg.LongName)
}

func (d *DefaultNode) GetImages(_ context.Context) map[string]string {
	return map[string]string{
		ImageKey: d.Cfg.Image,
	}
}

func (d *DefaultNode) GetRuntimeInformation(ctx context.Context) ([]types.GenericContainer, error) {
	genericContainers, err := d.GetRuntimeInformationBase(ctx)
	if err != nil {
		return nil, err
	}

	// if we did not get any generic container back, return early
	if len(genericContainers) == 0 {
		return genericContainers, nil
	}

	// populate ip information
	gcNwSettings := genericContainers[0].NetworkSettings
	if gcNwSettings != (types.GenericMgmtIPs{}) {
		d.Cfg.MgmtIPv4Address = gcNwSettings.IPv4addr
		d.Cfg.MgmtIPv4PrefixLength = gcNwSettings.IPv4pLen
		d.Cfg.MgmtIPv6Address = gcNwSettings.IPv6addr
		d.Cfg.MgmtIPv6PrefixLength = gcNwSettings.IPv6pLen
		d.Cfg.MgmtIPv4Gateway = gcNwSettings.IPv4Gw
		d.Cfg.MgmtIPv6Gateway = gcNwSettings.IPv6Gw
	}
	d.Cfg.ContainerID = genericContainers[0].ID

	return genericContainers, nil
}

func (d *DefaultNode) GetRuntimeInformationBase(ctx context.Context) ([]types.GenericContainer, error) {
	return d.Runtime.ListContainers(ctx, []*types.GenericFilter{
		{
			FilterType: "name",
			Match:      fmt.Sprintf("^%s$", d.Cfg.LongName), // this regexp ensure we have an exact match for name
		},
	})
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (d *DefaultNode) DeleteNetnsSymlink() error {
	log.Debugf("Deleting %s network namespace", d.Config().LongName)
	return utils.DeleteNetnsSymlink(d.Config().LongName)
}

func (d *DefaultNode) CheckInterfaceNamingConvention() error {
	log.Debugf("CheckInterfaceNamingConvention() not implemented for %q", d.Cfg.Kind)
	return nil
}

func (d *DefaultNode) VerifyStartupConfig(topoDir string) error {
	cfg := d.Config().StartupConfig
	if cfg == "" {
		return nil
	}

	rcfg := utils.ResolvePath(cfg, topoDir)
	if !utils.FileExists(rcfg) {
		return fmt.Errorf("node %q startup-config file not found by the path %s", d.Config().ShortName, rcfg)
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

// NodeOverwrites provides an interface used to be able to overwrite
// certain methods in the embedding struct
type NodeOverwrites interface {
	VerifyStartupConfig(topoDir string) error
	CheckInterfaceNamingConvention() error
	VerifyHostRequirements() error
	PullImage(ctx context.Context) error
	GetImages(ctx context.Context) map[string]string
	RunExecType(ctx context.Context, exec *types.Exec) (types.ExecReader, error)
}

func LoadStartupConfigFileVr(node Node, configDirName, startupCfgFName string) error {
	nodeCfg := node.Config()
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, configDirName), 0777)

	node.Config()
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

func (d *DefaultNode) RunExecConfig(ctx context.Context) ([]types.ExecReader, error) {
	result := []types.ExecReader{}
	for _, cmd := range d.Cfg.Exec {
		e, err := types.NewExec(cmd)
		if err != nil {
			return result, err
		}
		er, err := d.OverwriteNode.RunExecType(ctx, e)
		if err != nil {
			return nil, err
		}
		result = append(result, er)
	}
	return result, nil
}

// RunExecType is the final function that calls the runtime to execute a type.Exec on a container
// This is to be overriden if the nodes implementation differs
func (d *DefaultNode) RunExecType(ctx context.Context, exec *types.Exec) (types.ExecReader, error) {
	err := d.GetRuntime().Exec(ctx, d.Cfg.LongName, exec)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v", d.Cfg.LongName, exec.GetCmdString(), err)
		return nil, err
	}
	return exec, nil
}

// RunExecTypeWoWait is the final function that calls the runtime to execute a type.Exec on a container
// This is to be overriden if the nodes implementation differs
func (d *DefaultNode) RunExecTypeWoWait(ctx context.Context, exec *types.Exec) error {
	err := d.GetRuntime().ExecNotWait(ctx, d.Cfg.LongName, exec)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v", d.Cfg.LongName, exec.GetCmdString(), err)
		return err
	}
	return nil
}

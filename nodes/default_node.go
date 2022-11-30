// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"context"
	"fmt"

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

func (d *DefaultNode) PostDeploy(_ context.Context, _ map[string]Node, _ []types.GenericContainer) error {
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
	for imageKey, imageName := range d.GetImages(ctx) {
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
	genericContainer, err := d.GetRuntimeInformationBase(ctx)
	if err != nil {
		return nil, err
	}

	// if we did not get any generic container back, return early
	if len(genericContainer) == 0 {
		return genericContainer, nil
	}

	// populate ip information
	gcNwSettings := genericContainer[0].NetworkSettings
	if gcNwSettings != (types.GenericMgmtIPs{}) {
		d.Cfg.MgmtIPv4Address = gcNwSettings.IPv4addr
		d.Cfg.MgmtIPv4PrefixLength = gcNwSettings.IPv4pLen
		d.Cfg.MgmtIPv6Address = gcNwSettings.IPv6addr
		d.Cfg.MgmtIPv6PrefixLength = gcNwSettings.IPv6pLen
		d.Cfg.MgmtIPv4Gateway = gcNwSettings.IPv4Gw
		d.Cfg.MgmtIPv6Gateway = gcNwSettings.IPv6Gw
	}
	d.Cfg.ContainerID = genericContainer[0].ID

	return genericContainer, nil
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

// NodeOverwrites provides an interface used to be able to overwrite
// certain methods in the embedding struct
type NodeOverwrites interface {
	VerifyStartupConfig(topoDir string) error
	CheckInterfaceNamingConvention() error
	VerifyHostRequirements() error
	PullImage(ctx context.Context) error
}

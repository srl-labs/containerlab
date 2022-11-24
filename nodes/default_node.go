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
	Cfg     *types.NodeConfig
	Mgmt    *types.MgmtNet
	Runtime runtime.ContainerRuntime
}

func (d *DefaultNode) WithMgmtNet(mgmt *types.MgmtNet)                       { d.Mgmt = mgmt }
func (d *DefaultNode) WithRuntime(r runtime.ContainerRuntime)                { d.Runtime = r }
func (d *DefaultNode) GetRuntime() runtime.ContainerRuntime                  { return d.Runtime }
func (d *DefaultNode) Config() *types.NodeConfig                             { return d.Cfg }
func (d *DefaultNode) PreDeploy(_, _, _ string) error                        { return nil }
func (d *DefaultNode) PostDeploy(_ context.Context, _ map[string]Node) error { return nil }
func (d *DefaultNode) SaveConfig(_ context.Context) error {
	log.Debugf("Save operation is currently not supported for %q node kind", d.Cfg.Kind)
	return nil
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

func (d *DefaultNode) GetImages() map[string]string {
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
func (d *DefaultNode) DeleteNetnsSymlink() (err error) {
	log.Debugf("Deleting %s network namespace", d.Config().LongName)
	if err := utils.DeleteNetnsSymlink(d.Config().LongName); err != nil {
		return err
	}

	return nil
}

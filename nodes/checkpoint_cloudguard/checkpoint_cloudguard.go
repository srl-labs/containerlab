// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package checkpoint_cloudguard

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"checkpoint_cloudguard"}

const (
	scrapliPlatformName = "checkpoint_cloudguard"
	defaultUser         = "admin"
	defaultPassword     = "admin"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(CheckpointCloudguard)
	})
	err := nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
	if err != nil {
		log.Error(err)
	}
}

type CheckpointCloudguard struct {
	cfg     *types.NodeConfig
	mgmt    *types.MgmtNet
	runtime runtime.ContainerRuntime
}

func (n *CheckpointCloudguard) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	n.cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set startup arguments in boxen container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultUser,
		"PASSWORD":           defaultPassword,
		"DOCKER_NET_V4_ADDR": n.mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.mgmt.IPv6Subnet,
	}
	n.cfg.Env = utils.MergeStringMaps(defEnv, n.cfg.Env)

	n.cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.cfg.Env["USERNAME"], n.cfg.Env["PASSWORD"], n.cfg.ShortName, n.cfg.Env["CONNECTION_MODE"])

	// set virtualization requirement
	n.cfg.HostRequirements.VirtRequired = true

	return nil
}

func (n *CheckpointCloudguard) Config() *types.NodeConfig { return n.cfg }

func (n *CheckpointCloudguard) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(n.cfg.LabDir, 0777)
	return nil
}

func (n *CheckpointCloudguard) Deploy(ctx context.Context) error {
	cID, err := n.runtime.CreateContainer(ctx, n.cfg)
	if err != nil {
		return err
	}
	_, err = n.runtime.StartContainer(ctx, cID, n.cfg)
	return err
}

func (*CheckpointCloudguard) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return nil
}

func (s *CheckpointCloudguard) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (*CheckpointCloudguard) Destroy(_ context.Context) error          { return nil }
func (n *CheckpointCloudguard) WithMgmtNet(mgmt *types.MgmtNet)        { n.mgmt = mgmt }
func (n *CheckpointCloudguard) WithRuntime(r runtime.ContainerRuntime) { n.runtime = r }
func (n *CheckpointCloudguard) GetRuntime() runtime.ContainerRuntime   { return n.runtime }

func (n *CheckpointCloudguard) Delete(ctx context.Context) error {
	return n.runtime.DeleteContainer(ctx, n.cfg.LongName)
}

func (n *CheckpointCloudguard) SaveConfig(_ context.Context) error {
	// not implemented
	return nil
}

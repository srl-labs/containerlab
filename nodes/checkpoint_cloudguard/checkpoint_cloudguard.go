// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package checkpoint_cloudguard

import (
	"fmt"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindnames           = []string{"checkpoint_cloudguard"}
	defaultCredentials  = containerlabnodes.NewCredentials("admin", "admin")
	scrapliPlatformName = "notsupported"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, nil)
	r.Register(kindnames, func() containerlabnodes.Node {
		return new(CheckpointCloudguard)
	}, nrea)
}

type CheckpointCloudguard struct {
	containerlabnodes.VRNode
}

func (n *CheckpointCloudguard) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set startup arguments in boxen container
	defEnv := map[string]string{
		"CONNECTION_MODE":    containerlabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = containerlabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	return nil
}

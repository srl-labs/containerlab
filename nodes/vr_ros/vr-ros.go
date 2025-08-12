// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ros

import (
	"fmt"
	"path"
	"regexp"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"mikrotik_ros", "vr-ros", "vr-mikrotik_ros"}
	defaultCredentials = containerlabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`ether(?P<port>\d+)`)
	InterfaceOffset = 2
	InterfaceHelp   = "etherX (where X >= 2) or ethX (where X >= 1)"
)

const (
	configDirName   = "ftpboot"
	startupCfgFName = "config.auto.rsc"

	scrapliPlatformName = "mikrotik_routeros" //nolint: misspell
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	platformAttrs := &containerlabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() containerlabnodes.Node {
		return new(vrRos)
	}, nrea)
}

type vrRos struct {
	containerlabnodes.VRNode
}

func (n *vrRos) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	n.ConfigDirName = configDirName
	n.StartupCfgFName = startupCfgFName
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	defEnv := map[string]string{
		"CONNECTION_MODE":    containerlabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = containerlabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, "ftpboot"), ":/ftpboot"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

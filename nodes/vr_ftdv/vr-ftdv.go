// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ftdv

import (
	"fmt"
	"regexp"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"cisco_ftdv"}
	defaultCredentials = containerlabnodes.NewCredentials("admin", "Admin@123")

	InterfaceRegexp = regexp.MustCompile(`(?:GigabitEthernet|Gi)\s?0/(?P<port>\d+)`)
	InterfaceOffset = 0
	InterfaceHelp   = "GigabitEthernet0/X or Gi0/X (where X >= 0) or ethX (where X >= 1)"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "cisco_ftd"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	generateNodeAttributes := containerlabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &containerlabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformAttrs)

	r.Register(kindNames, func() containerlabnodes.Node {
		return new(vrFtdv)
	}, nrea)
}

type vrFtdv struct {
	containerlabnodes.VRNode
}

func (n *vrFtdv) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    containerlabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = containerlabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_xrv9k

import (
	"fmt"
	"path"
	"regexp"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"cisco_xrv9k", "vr-xrv9k", "vr-cisco_xrv9k"}
	defaultCredentials = clabnodes.NewCredentials("clab", "clab@123")

	InterfaceRegexp = regexp.MustCompile(`(?:Gi|GigabitEthernet|Te|TenGigE|TenGigabitEthernet)\s?0/0/0/(?P<port>\d+)`)
	InterfaceOffset = 0
	InterfaceHelp   = "GigabitEthernet0/0/0/X, Gi0/0/0/X or TenGigabitEthernet0/0/0/X, TenGigE0/0/0/X, Te0/0/0/X (where X >= 0) or ethX (where X >= 1)"
)

const (
	generateable     = true
	generateIfFormat = "Gi0/0/0/%d"

	scrapliPlatformName = "cisco_iosxr"
	NapalmPlatformName  = "iosxr"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformAttrs)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrXRV9K)
	}, nrea)
}

type vrXRV9K struct {
	clabnodes.VRNode
}

func (n *vrXRV9K) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"VCPU":               "2",
		"RAM":                "16384",
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --vcpu %s --ram %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"], n.Cfg.Env["VCPU"], n.Cfg.Env["RAM"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

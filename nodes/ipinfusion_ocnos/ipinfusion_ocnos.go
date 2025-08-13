// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ipinfusion_ocnos

import (
	"fmt"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"ipinfusion_ocnos"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin@123")
)

const (
	scrapliPlatformName = "ipinfusion_ocnos"
	generateable        = false
	generateIfFormat    = ""
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformAttrs)

	r.Register(kindnames, func() clabnodes.Node {
		return new(IPInfusionOcNOS)
	}, nrea)
}

type IPInfusionOcNOS struct {
	clabnodes.VRNode
}

func (n *IPInfusionOcNOS) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
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
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	return nil
}

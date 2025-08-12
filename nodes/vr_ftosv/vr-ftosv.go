// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ftosv

import (
	"fmt"
	"path"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"dell_ftosv", "vr-ftosv", "vr-dell_ftosv"}
	defaultCredentials = containerlabnodes.NewCredentials("admin", "admin")
)

const (
	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"

	scrapliPlatformName = "dell_emc"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	platformAttrs := &containerlabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() containerlabnodes.Node {
		return new(vrFtosv)
	}, nrea)
}

type vrFtosv struct {
	containerlabnodes.VRNode
}

func (n *vrFtosv) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, n.ScrapliPlatformName)
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

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	return nil
}

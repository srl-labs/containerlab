// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cjunosevolved

import (
	"fmt"
	"path"
	"regexp"

	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

var (
	kindnames          = []string{"cjunosevolved", "juniper_cjunosevolved"}
	defaultCredentials = containerlabnodes.NewCredentials("admin", "admin@123")
	InterfaceRegexp    = regexp.MustCompile(`et-0/0/(?P<port>\d+)$`)
	InterfaceOffset    = -3
	InterfaceHelp      = "(et-0/0/X (where X >= 0) or ethX (where X >= 4)"
)

const (
	scrapliPlatformName = "juniper_junos"
	NapalmPlatformName  = "junos"

	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"
)

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	platformAttrs := &containerlabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := containerlabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() containerlabnodes.Node {
		return new(cjunosevolved)
	}, nrea)
}

type cjunosevolved struct {
	containerlabnodes.VRNode
}

func (n *cjunosevolved) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *containerlabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)

	// cjunosevolved requires KVM support.
	n.HostRequirements.VirtRequired = true
	n.HostRequirements.MinVCPU = 4
	n.HostRequirements.MinVCPUFailAction = containerlabtypes.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 8
	n.HostRequirements.MinAvailMemoryGbFailAction = containerlabtypes.FailBehaviourLog

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

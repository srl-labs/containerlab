// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cjunosevolved

import (
	"fmt"
	"path"
	"regexp"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

var (
	kindnames          = []string{"cjunosevolved", "juniper_cjunosevolved"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin@123")
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
func Register(r *clabnodes.NodeRegistry) {
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() clabnodes.Node {
		return new(cjunosevolved)
	}, nrea)
}

type cjunosevolved struct {
	clabnodes.VRNode
}

func (n *cjunosevolved) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)

	// cjunosevolved requires KVM support.
	n.HostRequirements.VirtRequired = true
	n.HostRequirements.MinVCPU = 4
	n.HostRequirements.MinVCPUFailAction = clabtypes.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 8
	n.HostRequirements.MinAvailMemoryGbFailAction = clabtypes.FailBehaviourLog

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

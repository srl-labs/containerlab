// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cjunosevolved

import (
	"fmt"
	"path"
	"regexp"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

var (
	kindnames          = []string{"cjunosevolved", "juniper_cjunosevolved"}
	defaultCredentials = nodes.NewCredentials("admin", "admin@123")
	InterfaceRegexp    = regexp.MustCompile(`et-0/0/(?P<port>\d+)$`)
	InterfaceOffset    = 0
	InterfaceHelp      = "(et-0/0/X (where X >= 0) or ethX (where X >= 4)"
)

const (
	scrapliPlatformName = "juniper_junos"
	NapalmPlatformName  = "junos"

	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	platformAttrs := &nodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindnames, func() nodes.Node {
		return new(cjunosevolved)
	}, nrea)
}

type cjunosevolved struct {
	nodes.DefaultNode
}

func (n *cjunosevolved) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	// cjunosevolved requires KVM support.
	n.HostRequirements.VirtRequired = true
	n.HostRequirements.MinVCPU = 4
	n.HostRequirements.MinVCPUFailAction = types.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 8
	n.HostRequirements.MinAvailMemoryGbFailAction = types.FailBehaviourLog

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"))

	return nil
}

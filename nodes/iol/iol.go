// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cisco_iol

import (
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"cisco_iol"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`(?:e|Ethernet)\s?0/(?P<port>\d+)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "e0/X or Ethernet0/X (where X >= 1) or ethX (where X >= 1)"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(iol)
	}, defaultCredentials)
}

type iol struct {
	nodes.VRNode
}

func (n *iol) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *nodes.NewVRNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// mount nvram so that config persists
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, "nvram"), ":/opt/iol/nvram_00001"))

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (s *iol) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)

	if !utils.FileExists(path.Join(s.Cfg.LabDir, "nvram")) {
		// create nvram file
		utils.CreateFile(path.Join(s.Cfg.LabDir, "nvram"), "")
	}

	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return err
}

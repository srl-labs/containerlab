// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package checkpoint_cloudguard

import (
	"context"
	"fmt"
	"regexp"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"checkpoint_cloudguard"}
var defaultCredentials = nodes.NewCredentials("admin", "admin")

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(CheckpointCloudguard)
	}, defaultCredentials)
}

type CheckpointCloudguard struct {
	nodes.DefaultNode
}

func (n *CheckpointCloudguard) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	// env vars are used to set startup arguments in boxen container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = utils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"], n.Cfg.Env["PASSWORD"], n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	return nil
}

func (n *CheckpointCloudguard) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(n.Cfg.LabDir, 0777)
	return nil
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *CheckpointCloudguard) CheckInterfaceName() error {
	// allow eth and et interfaces
	// https://regex101.com/r/C3Fhr0/1
	ifRe := regexp.MustCompile(`eth[1-9]+$`)
	for _, e := range n.Config().Endpoints {
		if !ifRe.MatchString(e.EndpointName) {
			return fmt.Errorf("%q interface name %q doesn't match the required pattern. It should be named as ethX, where X is >1", n.Cfg.ShortName, e.EndpointName)
		}
	}

	return nil
}

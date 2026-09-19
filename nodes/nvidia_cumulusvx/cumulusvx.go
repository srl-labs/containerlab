// Copyright 2026 NVIDIA
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nvidia_cumulusvx

import (
	"context"
	"fmt"
	"path"
	"regexp"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"nvidia_cumulusvx"}
	defaultCredentials = clabnodes.NewCredentials("cumulus", "Clab123!")

	InterfaceRegexp = regexp.MustCompile(`^swp(?P<port>[1-9]\d*)(?:s(?P<lane>\d+))?$`)
	InterfaceOffset = 1
	InterfaceHelp   = "swpN or ethN (N >= 1), or swpNsM with extras.cumulus-vx breakouts"
)

const (
	generateable     = true
	generateIfFormat = "swp%d"
	configDirName    = "config"
)

func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		nil,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(nvidiaCumulusVX)
	}, nrea)
}

type nvidiaCumulusVX struct {
	clabnodes.VRNode
	portLayout *portLayout
}

func (n *nvidiaCumulusVX) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, "")
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	var err error
	n.portLayout, err = newPortLayout(cumulusExtras(cfg))
	if err != nil {
		return fmt.Errorf("node %q: %w", cfg.ShortName, err)
	}

	defEnv := map[string]string{
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/config"),
	)

	n.Cfg.Cmd = fmt.Sprintf(
		"--username %s --password %s --hostname %s --connection-mode %s --trace",
		n.Cfg.Env["USERNAME"],
		n.Cfg.Env["PASSWORD"],
		n.Cfg.ShortName,
		n.Cfg.Env["CONNECTION_MODE"],
	)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *nvidiaCumulusVX) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	if err := n.VRNode.PreDeploy(ctx, params); err != nil {
		return err
	}
	return n.writePortsConfig()
}

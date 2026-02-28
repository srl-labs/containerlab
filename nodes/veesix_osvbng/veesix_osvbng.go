// Copyright 2025 Veesix Networks
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package veesix_osvbng

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	generateable     = true
	generateIfFormat = "eth%d"

	osvbngCfgDstPath = "/etc/osvbng/osvbng.yaml"
)

var kindNames = []string{"veesix_osvbng"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(veesix_osvbng)
	}, nrea)
}

type veesix_osvbng struct {
	clabnodes.DefaultNode
	// Path to the osvbng config file on the host
	osvbngCfgSrcPath string
}

func (n *veesix_osvbng) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// Capabilities required by osvbng (VPP-based, needs hugepages, raw I/O)
	n.Cfg.CapAdd = append(n.Cfg.CapAdd,
		"SYS_ADMIN",
		"NET_ADMIN",
		"IPC_LOCK",
		"SYS_NICE",
		"SYS_RAWIO",
	)

	// Tell the entrypoint to wait for containerlab to attach all interfaces
	if n.Cfg.Env == nil {
		n.Cfg.Env = make(map[string]string)
	}
	n.Cfg.Env["OSVBNG_WAIT_FOR_INTERFACES"] = "true"

	// Set up the osvbng config bind mount
	n.osvbngCfgSrcPath = path.Join(n.Cfg.LabDir, "osvbng.yaml")
	n.Cfg.ResStartupConfig = n.osvbngCfgSrcPath
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.Cfg.ResStartupConfig, ":", osvbngCfgDstPath))

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *veesix_osvbng) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	nodeCfg := n.Config()

	// Handle startup config provided by the user
	var osvbngCfgTpl string

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		osvbngCfgTpl = string(c)
	}

	err := n.GenerateConfig(n.Cfg.ResStartupConfig, osvbngCfgTpl)
	if err != nil {
		return err
	}

	return nil
}

// CheckInterfaceName allows any interface name for osvbng nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *veesix_osvbng) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)
	for _, e := range n.Endpoints {
		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf(
				"eth0 interface name is not allowed for %s node when network mode is not set to none",
				n.Cfg.ShortName,
			)
		}
	}
	return nil
}

func (n *veesix_osvbng) GetImages(_ context.Context) map[string]string {
	return map[string]string{
		clabnodes.ImageKey: n.Cfg.Image,
	}
}

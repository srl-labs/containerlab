// Copyright 2025 Code Laboratory Ltd
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package codelaboratory_bng

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

	bngCfgDstPath       = "/etc/bng/config.yaml"
	ifWaitScriptDstPath = "/usr/sbin/if-wait.sh"
)

var kindNames = []string{"codelaboratory_bng"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(nil, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(codelaboratory_bng)
	}, nrea)
}

type codelaboratory_bng struct {
	clabnodes.DefaultNode
	// Path to the BNG config file on the host
	bngCfgSrcPath string
	// Path to the interface wait script on the host
	ifWaitSrcPath string
}

func (n *codelaboratory_bng) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = cfg

	// Capabilities required by BNG (eBPF/XDP - much lighter than VPP/DPDK)
	n.Cfg.CapAdd = append(n.Cfg.CapAdd,
		"NET_ADMIN",
		"BPF",
	)

	// Tell the BNG which interface to use for subscriber traffic
	if n.Cfg.Env == nil {
		n.Cfg.Env = make(map[string]string)
	}

	// Set up the BNG config bind mount
	n.bngCfgSrcPath = path.Join(n.Cfg.LabDir, "config.yaml")
	n.Cfg.ResStartupConfig = n.bngCfgSrcPath
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.Cfg.ResStartupConfig, ":", bngCfgDstPath))

	// Bind-mount the interface wait script so the container can wait for
	// containerlab to wire the veth links before starting the BNG process.
	n.ifWaitSrcPath = path.Join(n.Cfg.LabDir, "if-wait.sh")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(n.ifWaitSrcPath, ":", ifWaitScriptDstPath))

	// Override the entrypoint to wait for interfaces before starting BNG.
	// The BNG image has ENTRYPOINT ["/app/bng"] CMD ["run"], but containerlab
	// creates containers first and wires links afterwards. Without this wrapper,
	// the BNG process tries to bind its subscriber-facing interface (eth1)
	// before the veth pair exists and crashes.
	n.Cfg.Entrypoint = "sh -c '" + ifWaitScriptDstPath + " ; exec /app/bng run --config " + bngCfgDstPath + "'"

	for _, o := range opts {
		o(n)
	}

	return nil
}

func (n *codelaboratory_bng) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)

	// Generate the interface wait script from the shared template.
	// This script polls /sys/class/net/ until all CLAB_INTFS interfaces
	// appear, ensuring the BNG doesn't start before its links are wired.
	clabutils.CreateFile(n.ifWaitSrcPath, clabutils.IfWaitScript)
	os.Chmod(n.ifWaitSrcPath, clabconstants.PermissionsOpen)

	nodeCfg := n.Config()

	// Handle startup config provided by the user
	var bngCfgTpl string

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		bngCfgTpl = string(c)
	}

	err := n.GenerateConfig(n.Cfg.ResStartupConfig, bngCfgTpl)
	if err != nil {
		return err
	}

	return nil
}

// CheckInterfaceName allows any interface name for BNG nodes, but checks
// if eth0 is only used with network-mode=none.
func (n *codelaboratory_bng) CheckInterfaceName() error {
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

func (n *codelaboratory_bng) GetImages(_ context.Context) map[string]string {
	return map[string]string{
		clabnodes.ImageKey: n.Cfg.Image,
	}
}

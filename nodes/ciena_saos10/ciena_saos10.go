// Copyright 2026 Ciena Corporation
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ciena_saos10

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"ciena_saos10", "vr-ciena_saos10"}
	defaultCredentials = clabnodes.NewCredentials("diag", "ciena123")

	InterfaceRegexp = regexp.MustCompile(`^(?P<port>[1-9]\d*)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "X (where X >= 1) or ethX (where X >= 1)"

	// supportedVariantList enumerates the SAOS chassis variants accepted in the
	// topology "type" field. Keep it in sync with the vrnetlab SAOS image
	// variant_map (saos/docker/launch.py).
	supportedVariantList = []string{
		"3948", "3949", "3984", "3985",
		"5130", "5131", "5131-910", "5132", "5134", "5144",
		"5162", "5164", "5164-902", "5166", "5166-903", "5168",
		"5169", "5170", "5171", "5171-920", "5184", "5186",
		"8110", "8112", "8114", "8140", "8190", "8192",
	}

	supportedVariants = variantSet(supportedVariantList)
)

// variantSet builds a lookup set from the supported-variant list so the slice
// and the set used for validation cannot drift apart.
func variantSet(variants []string) map[string]struct{} {
	m := make(map[string]struct{}, len(variants))
	for _, v := range variants {
		m[v] = struct{}{}
	}
	return m
}

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(cienaSaos10)
	}, nrea)
}

type cienaSaos10 struct {
	clabnodes.VRNode
}

func (n *cienaSaos10) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, "")
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	variant := strings.TrimSpace(n.Cfg.NodeType)
	if variant == "" {
		return fmt.Errorf("missing type for ciena_saos10 node %q", n.Cfg.ShortName)
	}
	if _, ok := supportedVariants[variant]; !ok {
		return fmt.Errorf(
			"unsupported type %q for ciena_saos10 node %q. Supported: %s",
			variant,
			n.Cfg.ShortName,
			strings.Join(supportedVariantList, ", "),
		)
	}

	defEnv := map[string]string{
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

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

func (n *cienaSaos10) VerifyStartupConfig(topoDir string) error {
	if err := n.VRNode.VerifyStartupConfig(topoDir); err != nil {
		return err
	}
	if n.Cfg.StartupConfig == "" {
		return nil
	}
	if !clabutils.IsPartialConfigFile(n.Cfg.StartupConfig) {
		return fmt.Errorf(
			"ciena_saos10 node %q only supports partial startup-config files (missing .partial): %s",
			n.Cfg.ShortName,
			n.Cfg.StartupConfig,
		)
	}
	return nil
}

func (n *cienaSaos10) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	if n.Cfg.StartupConfig != "" {
		if !clabutils.IsPartialConfigFile(n.Cfg.StartupConfig) {
			return fmt.Errorf(
				"ciena_saos10 node %q only supports partial startup-config files (missing .partial): %s",
				n.Cfg.ShortName,
				n.Cfg.StartupConfig,
			)
		}
		n.StartupCfgFName = filepath.Base(n.Cfg.StartupConfig)
		if n.Cfg.Env == nil {
			n.Cfg.Env = map[string]string{}
		}
		n.Cfg.Env["SAOS_STARTUP_CONFIG_PATH"] = path.Join("/config", n.StartupCfgFName)
	}
	return n.VRNode.PreDeploy(ctx, params)
}

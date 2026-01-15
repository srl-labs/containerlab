// Copyright 2024 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ciena_saos

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"ciena_saos", "vr-ciena_saos"}
	defaultCredentials = clabnodes.NewCredentials("diag", "ciena123")

	InterfaceRegexp = regexp.MustCompile(`^(?P<port>[1-9]\d*)$`)
	InterfaceOffset = 1
	InterfaceHelp   = "X (where X >= 1) or ethX (where X >= 1)"

	supportedVariantList = []string{
		"3948",
		"3984",
		"3985",
		"5130",
		"5131",
		"5132",
		"5134",
		"5144",
		"5162",
		"5164",
		"5166",
		"5168",
		"5169",
		"5170",
		"5171",
		"5184",
		"5186",
		"8110",
		"8112",
		"8114",
		"8140",
		"8190",
		"8192",
	}

	supportedVariants = map[string]struct{}{
		"3948": {},
		"3984": {},
		"3985": {},
		"5130": {},
		"5131": {},
		"5132": {},
		"5134": {},
		"5144": {},
		"5162": {},
		"5164": {},
		"5166": {},
		"5168": {},
		"5169": {},
		"5170": {},
		"5171": {},
		"5184": {},
		"5186": {},
		"8110": {},
		"8112": {},
		"8114": {},
		"8140": {},
		"8190": {},
		"8192": {},
	}
)

const (
	generateable     = true
	generateIfFormat = "eth%d"
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, nil)

	r.Register(kindNames, func() clabnodes.Node {
		return new(cienaSaos)
	}, nrea)
}

type cienaSaos struct {
	clabnodes.VRNode
}

func (n *cienaSaos) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, "")
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	variant := strings.TrimSpace(n.Cfg.NodeType)
	if variant == "" {
		return fmt.Errorf("missing type for ciena_saos node %q", n.Cfg.ShortName)
	}
	if _, ok := supportedVariants[variant]; !ok {
		return fmt.Errorf(
			"unsupported type %q for ciena_saos node %q. Supported: %s",
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

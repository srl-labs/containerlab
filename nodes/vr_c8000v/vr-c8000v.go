// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_c8000v

import (
	"fmt"
	"path"
	"regexp"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"cisco_c8000v"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`(?:Gi|GigabitEthernet)\s?(?P<port>\d+)$`)
	InterfaceOffset = 2
	InterfaceHelp   = "GiX or GigabitEthernetX (where X >= 2) or ethX (where X >= 1)"
)

const (
	scrapliPlatformName = "cisco_iosxe"

	generateable     = true
	generateIfFormat = "eth%d"

	// C8000v modes.
	modeAutonomous = "autonomous"
	modeController = "controller"

	// Default mode if not specified.
	defaultMode = modeAutonomous
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformAttrs,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrC8000v)
	}, nrea)
}

type vrC8000v struct {
	clabnodes.VRNode
	mode string
}

func (n *vrC8000v) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// Determine mode from NodeType field
	// NodeType can be: autonomous, controller
	// If not specified, defaults to autonomous
	n.mode = n.Cfg.NodeType
	if n.mode == "" {
		n.mode = defaultMode
	}

	// Validate mode
	if !isValidMode(n.mode) {
		return fmt.Errorf(
			"invalid mode %q for cisco_c8000v node %q. Must be one of: autonomous, controller",
			n.mode,
			n.Cfg.ShortName,
		)
	}

	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
		"MODE":               n.mode, // Set MODE environment variable for vrnetlab
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	// mount config dir to support startup-config functionality
	n.Cfg.Binds = append(
		n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, n.ConfigDirName), ":/config"),
	)

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
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

// isValidMode checks if the mode is valid.
func isValidMode(mode string) bool {
	switch mode {
	case modeAutonomous, modeController:
		return true
	default:
		return false
	}
}

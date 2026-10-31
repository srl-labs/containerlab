// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ros

import (
	"context"
	"fmt"
	"net"
	"path"
	"regexp"
	"slices"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	kindNames          = []string{"mikrotik_ros", "vr-ros", "vr-mikrotik_ros"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`ether(?P<port>\d+)`)
	InterfaceOffset = 2
	InterfaceHelp   = "etherX (where X >= 2) or ethX (where X >= 1)"
)

const (
	configDirName   = "ftpboot"
	startupCfgFName = "config.auto.rsc"

	scrapliPlatformName = "mikrotik_routeros" //nolint: misspell
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindNames, func() clabnodes.Node {
		return new(vrRos)
	}, nrea)
}

type vrRos struct {
	clabnodes.VRNode
}

func (n *vrRos) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init VRNode
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	n.ConfigDirName = configDirName
	n.StartupCfgFName = startupCfgFName
	// set virtualization requirement
	n.HostRequirements.VirtRequired = true

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	defEnv := map[string]string{
		"CONNECTION_MODE":    clabnodes.VrDefConnMode,
		"USERNAME":           n.Cfg.Credentials.Username,
		"PASSWORD":           n.Cfg.Credentials.Password,
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, "ftpboot"), ":/ftpboot"))

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

// SaveConfig overrides the default VRNode SaveConfig to handle MikroTik RouterOS
// Uses direct SSH connection since scrapligo doesn't support MikroTik RouterOS platform.
// To be refactored to use scrapli's GenericDriver or an enhanced scraplicfg.
func (n *vrRos) SaveConfig(_ context.Context) (*clabnodes.SaveConfigResult, error) {
	configContent, err := clabnodes.ExportConfigOverSSH(
		net.JoinHostPort(n.Cfg.LongName, "22"),
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
		"export",
	)
	if err != nil {
		return nil, err
	}

	// Filter out ether1 management interface IP address configuration
	filteredConfig := n.filterManagementInterfaceConfig(configContent)

	return clabnodes.WriteSavedConfig(n.Cfg, n.ConfigDirName, n.StartupCfgFName,
		[]byte(filteredConfig))
}

// filterManagementInterfaceConfig removes ether1 (management interface) IP address configuration
// from the exported RouterOS configuration to avoid including containerlab management IP settings.
func (n *vrRos) filterManagementInterfaceConfig(config string) string {
	lines := strings.Split(config, "\n")
	filteredLines := make([]string, 0, len(lines))
	inIPAddrSection := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "/") {
			inIPAddrSection = trimmedLine == "/ip address"
		}

		if inIPAddrSection &&
			strings.HasPrefix(trimmedLine, "add address=") &&
			slices.Contains(strings.Fields(trimmedLine), "interface=ether1") {
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	return strings.Join(filteredLines, "\n")
}

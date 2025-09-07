// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_ros

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
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
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"DOCKER_NET_V4_ADDR": n.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": n.Mgmt.IPv6Subnet,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(path.Join(n.Cfg.LabDir, "ftpboot"), ":/ftpboot"))

	if n.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		n.Cfg.Binds = append(n.Cfg.Binds, "/dev:/dev")
	}

	n.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), n.Cfg.ShortName, n.Cfg.Env["CONNECTION_MODE"])

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

// SaveConfig overrides the default VRNode SaveConfig to handle MikroTik RouterOS
// Uses direct SSH connection since scrapligo doesn't support MikroTik RouterOS platform.
// To be refactored to use scrapli's GenericDriver or an enhanced scraplicfg.
func (n *vrRos) SaveConfig(_ context.Context) error {
	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User: n.Credentials.GetUsername(),
		Auth: []ssh.AuthMethod{
			ssh.Password(n.Credentials.GetPassword()),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Accept any host key
		Timeout:         10 * time.Second,
	}

	// Connect to the MikroTik RouterOS device
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", n.Cfg.LongName), config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s via SSH: %+v", n.Cfg.LongName, err)
	}
	defer conn.Close()

	// Create a session
	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session for %s: %+v", n.Cfg.LongName, err)
	}
	defer session.Close()

	// Execute the export command to get the configuration
	output, err := session.CombinedOutput("export")
	if err != nil {
		return fmt.Errorf("failed to execute export command on %s: %+v", n.Cfg.LongName, err)
	}

	config_content := strings.TrimSpace(string(output))
	if config_content == "" {
		return fmt.Errorf("received empty configuration from %s", n.Cfg.LongName)
	}

	// Filter out ether1 management interface IP address configuration
	filtered_config := n.filterManagementInterfaceConfig(config_content)

	// Save config to mounted labdir startup config path
	configPath := filepath.Join(n.Cfg.LabDir, n.ConfigDirName, n.StartupCfgFName)
	err = os.WriteFile(configPath, []byte(filtered_config),
		clabconstants.PermissionsOpen) // skipcq: GO-S2306
	if err != nil {
		return fmt.Errorf("failed to write config by %s path from %s container: %v", configPath, n.Cfg.ShortName, err)
	}
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return nil
}

// filterManagementInterfaceConfig removes ether1 (management interface) IP address configuration
// from the exported RouterOS configuration to avoid including containerlab management IP settings.
func (n *vrRos) filterManagementInterfaceConfig(config string) string {
	lines := strings.Split(config, "\n")
	var filteredLines []string

	for _, line := range lines {
		// Skip lines related to ether1 IP address configuration
		if strings.Contains(line, "/ip address") {
			// Mark that we're in the IP address section
			filteredLines = append(filteredLines, line)
			continue
		}

		// Skip IP address entries for ether1 interface
		if strings.Contains(line, "interface=ether1") &&
			(strings.Contains(line, "add address=") || strings.Contains(line, "add ")) {
			// Skip this line as it's ether1 IP configuration
			continue
		}

		// Keep all other lines
		filteredLines = append(filteredLines, line)
	}

	return strings.Join(filteredLines, "\n")
}

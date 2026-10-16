// Copyright 2026 NetPilot Labs
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package mikrotik_chr runs MikroTik RouterOS CHR from the official mikrotik/chr image.
//
// The image is MikroTik's own distribution (Alpine + qemu + the CHR disk) and is used
// unmodified: the kind writes chr-launch.sh into the node's lab directory, bind-mounts it
// read-only and sets it as the entrypoint in place of the image's /start.sh, which is not
// orchestration-ready (containerlab#3116, #3464).
package mikrotik_chr

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
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
	kindNames          = []string{"mikrotik_chr"}
	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`ether(?P<port>\d+)`)
	InterfaceOffset = 2
	InterfaceHelp   = "etherX (where X >= 2) or ethX (where X >= 1)"

	//go:embed chr-launch.sh
	launcherScript []byte
)

const (
	configDirName   = "ftpboot"
	startupCfgFName = "config.auto.rsc"

	launcherFName = "chr-launch.sh"
	launcherPath  = "/chr-launch.sh"
	// readyFile is written by the launcher once RouterOS accepted the credentials and the
	// startup config, and removed at every container start.
	readyFile = "/.chr-launch-ready"

	// mgmtComment tags every management object the launcher creates in RouterOS.
	mgmtComment = "clab-mgmt"

	scrapliPlatformName = "mikrotik_routeros" //nolint: misspell
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(defaultCredentials, nil, platformAttrs)

	r.Register(kindNames, func() clabnodes.Node {
		return new(mikrotikCHR)
	}, nrea)
}

type mikrotikCHR struct {
	clabnodes.VRNode
}

func (n *mikrotikCHR) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	n.VRNode = *clabnodes.NewVRNode(n, defaultCredentials, scrapliPlatformName)
	n.ConfigDirName = configDirName
	n.StartupCfgFName = startupCfgFName
	// The image's qemu falls back to TCG emulation without /dev/kvm: KVM makes the boot
	// fast but is not required.
	n.HostRequirements.VirtRequired = false

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}

	// Credentials travel as env, not on the command line, so a password needs no quoting.
	defEnv := map[string]string{
		"USERNAME": n.Cfg.Credentials.Username,
		"PASSWORD": n.Cfg.Credentials.Password,
	}
	n.Cfg.Env = clabutils.MergeStringMaps(defEnv, n.Cfg.Env)

	n.Cfg.Binds = append(n.Cfg.Binds,
		fmt.Sprint(path.Join(n.Cfg.LabDir, configDirName), ":/ftpboot"),
		fmt.Sprint(path.Join(n.Cfg.LabDir, launcherFName), ":", launcherPath, ":ro"),
	)

	if n.Cfg.Entrypoint == "" {
		n.Cfg.Entrypoint = "/bin/sh " + launcherPath
	}
	n.Cfg.Cmd = fmt.Sprintf("--hostname %s", n.Cfg.ShortName)

	if n.Cfg.Healthcheck == nil {
		n.Cfg.Healthcheck = &clabtypes.HealthcheckConfig{
			Test:        []string{"CMD-SHELL", "test -f " + readyFile},
			Interval:    5,
			Timeout:     3,
			Retries:     3,
			StartPeriod: 300,
		}
	}

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp

	return nil
}

// PreDeploy writes the launcher next to the startup config on every deploy, so a
// containerlab upgrade ships its launcher to existing labs.
func (n *mikrotikCHR) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	if err := n.VRNode.PreDeploy(ctx, params); err != nil {
		return err
	}

	dst := filepath.Join(n.Cfg.LabDir, launcherFName)
	if err := os.WriteFile(dst, launcherScript, clabconstants.PermissionsFileDefault); err != nil {
		return fmt.Errorf("node %q: failed to write %s: %w", n.Cfg.ShortName, dst, err)
	}

	return nil
}

// PreStop skips VRNode's vrnetlab qcow2 alias step, which does not apply to this image.
func (*mikrotikCHR) PreStop(_ context.Context) error {
	return nil
}

// SaveConfig exports the RouterOS configuration over SSH (scrapligo has no RouterOS
// platform) and writes it as the node's startup config, without the objects the launcher
// creates at every boot.
func (n *mikrotikCHR) SaveConfig(_ context.Context) (*clabnodes.SaveConfigResult, error) {
	config := &ssh.ClientConfig{
		User:            n.Cfg.Credentials.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(n.Cfg.Credentials.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lab node, host key unknown
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", n.Cfg.LongName), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s via SSH: %w", n.Cfg.LongName, err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session for %s: %w", n.Cfg.LongName, err)
	}
	defer session.Close()

	output, err := session.CombinedOutput("/export")
	if err != nil {
		return nil, fmt.Errorf("failed to export the configuration of %s: %w", n.Cfg.LongName, err)
	}

	exported := strings.TrimSpace(string(output))
	if exported == "" {
		return nil, fmt.Errorf("received empty configuration from %s", n.Cfg.LongName)
	}

	configPath := filepath.Join(n.Cfg.LabDir, n.ConfigDirName, n.StartupCfgFName)
	err = os.WriteFile(configPath, []byte(filterLauncherConfig(exported)+"\n"),
		clabconstants.PermissionsOpen) // skipcq: GO-S2306
	if err != nil {
		return nil, fmt.Errorf("failed to write config by %s path from %s container: %w",
			configPath, n.Cfg.ShortName, err)
	}
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return &clabnodes.SaveConfigResult{ConfigPath: configPath}, nil
}

// filterLauncherConfig drops what the launcher configures at every boot from an /export:
// every object tagged clab-mgmt (the ether1 management addresses, the mgmt routing table,
// its routes and rules) and CHR's ether1 DHCP client. Imported again as a startup config,
// these lines duplicate objects the launcher already created and RouterOS stops the import
// at the first one; the management addressing also belongs to the deployment, not the lab.
// Continuation lines (a trailing backslash) are joined before matching.
func filterLauncherConfig(config string) string {
	var out []string

	section := ""
	pending := ""
	for line := range strings.SplitSeq(strings.ReplaceAll(config, "\r\n", "\n"), "\n") {
		if pending != "" {
			line = pending + "\n" + line
			pending = ""
		}
		if strings.HasSuffix(line, `\`) {
			pending = line
			continue
		}

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/") {
			section = trimmed
		}
		if launcherOwned(section, trimmed) {
			continue
		}
		out = append(out, line)
	}
	if pending != "" {
		out = append(out, pending)
	}

	return strings.Join(out, "\n")
}

func launcherOwned(section, line string) bool {
	if !strings.HasPrefix(line, "add ") {
		return false
	}

	fields := strings.Fields(strings.ReplaceAll(line, `\`, " "))
	has := func(kv string) bool { return slices.Contains(fields, kv) }

	if has("comment=" + mgmtComment) {
		return true
	}

	switch section {
	case "/ip address", "/ipv6 address", "/ip dhcp-client":
		return has("interface=ether1")
	}

	return false
}

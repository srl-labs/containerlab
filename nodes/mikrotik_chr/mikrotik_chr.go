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
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
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

	// amd64's tested PCI-bridge layout. Beyond it the guest runs out of interrupt
	// vectors (about 100 NICs per vCPU) and SeaBIOS out of PCI I/O space (8 bridges).
	maxDataNICIndex = 63
	// arm64's virt machine gets no PCI bridges: its single bus holds the disk, the
	// management NIC and at most 29 data NICs before QEMU runs out of slots.
	maxDataNICIndexARM64 = 29

	// the launcher's CHR_INTF_WAIT and CHR_BOOT_WAIT defaults, and the time its config
	// fetch and gateway check add, in seconds.
	defaultIntfWait  = 60
	defaultBootWait  = 300
	startPeriodSlack = 90
	// the launcher's wait for the interface list to settle and its re-plug rounds, in seconds.
	replugWait = 180

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
	// goarch selects the NIC layout the interface limit is checked against. The
	// containerlab binary's architecture matches the image's outside emulated setups.
	goarch string
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
			StartPeriod: startPeriod(n.Cfg.Env),
		}
	}

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceOffset = InterfaceOffset
	n.InterfaceHelp = InterfaceHelp
	n.goarch = runtime.GOARCH

	return nil
}

// dataNICLimit is the highest data NIC index the launcher's NIC layout holds on goarch.
func dataNICLimit(goarch string) int {
	if goarch == "arm64" {
		return maxDataNICIndexARM64
	}

	return maxDataNICIndex
}

// CheckInterfaceName rejects data NIC indices the launcher does not create: beyond the
// tested PCI-bridge layout on amd64, or the free PCI slots on arm64.
func (n *mikrotikCHR) CheckInterfaceName() error {
	if err := n.VRNode.CheckInterfaceName(); err != nil {
		return err
	}

	limit := dataNICLimit(n.goarch)
	for _, ep := range n.Endpoints {
		ifName := ep.GetIfaceName()
		// Name the interface as written in the topology (etherN), with its ethN mapping.
		shown := fmt.Sprintf("%q", ep.GetIfaceDisplayName())
		if ep.GetIfaceDisplayName() != ifName {
			shown += " (" + ifName + ")"
		}
		index, err := strconv.Atoi(strings.TrimPrefix(ifName, "eth"))
		// An index too large for an int is still an index over the limit.
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return fmt.Errorf(
				"node %q interface %s: cannot parse interface index: %w",
				n.Cfg.ShortName, shown, err,
			)
		}
		if err != nil || index > limit {
			return fmt.Errorf(
				"node %q interface %s exceeds the maximum data NIC index %d on %s (eth%d / ether%d)",
				n.Cfg.ShortName,
				shown,
				limit,
				n.goarch,
				limit,
				limit+1,
			)
		}
	}

	return nil
}

// startPeriod is the healthcheck start period in seconds: the launcher's own worst case
// before the ready file, so a slow boot (TCG) never reports unhealthy. It waits up to
// CHR_INTF_WAIT for the links, then up to CHR_BOOT_WAIT each for REST, the startup-config
// import and the SSH banner, plus the config fetch, the gateway check and re-plugging NICs
// RouterOS rejected.
func startPeriod(env map[string]string) int {
	return envSeconds(env, "CHR_INTF_WAIT", defaultIntfWait) +
		3*envSeconds(env, "CHR_BOOT_WAIT", defaultBootWait) + startPeriodSlack + replugWait
}

// envSeconds reads a positive number of seconds from env, else returns def (the launcher
// falls back to its default the same way).
func envSeconds(env map[string]string, key string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(env[key])); err == nil && v > 0 {
		return v
	}

	return def
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
// platform, see clabnodes.ExportConfigOverSSH) and writes it as the node's startup config,
// without the objects the launcher creates at every boot.
func (n *mikrotikCHR) SaveConfig(_ context.Context) (*clabnodes.SaveConfigResult, error) {
	exported, err := clabnodes.ExportConfigOverSSH(
		net.JoinHostPort(n.Cfg.LongName, "22"),
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
		"/export",
	)
	if err != nil {
		return nil, err
	}

	return clabnodes.WriteSavedConfig(n.Cfg, n.ConfigDirName, n.StartupCfgFName,
		[]byte(filterLauncherConfig(exported)+"\n"))
}

// filterLauncherConfig drops what the launcher configures at every boot from an /export:
// every object tagged clab-mgmt (the ether1 management addresses, the mgmt routing table,
// its routes and rules), the ether1 ND entry and CHR's ether1 DHCP client. Imported again
// as a startup config, these lines duplicate objects the launcher already created and
// RouterOS stops the import at the first one; the management addressing also belongs to
// the deployment, not the lab. Continuation lines (an unescaped trailing backslash) are joined
// before
// matching.
func filterLauncherConfig(config string) string {
	var out []string

	section := ""
	pending := ""
	for line := range strings.SplitSeq(strings.ReplaceAll(config, "\r\n", "\n"), "\n") {
		if pending != "" {
			line = pending + "\n" + line
			pending = ""
		}
		if continues(line) {
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

// continues reports whether an /export line continues on the next one: it ends in an
// unescaped backslash. RouterOS writes a literal backslash in a value as `\\`, so an even
// run of trailing backslashes ends the line.
func continues(line string) bool {
	trailing := len(line) - len(strings.TrimRight(line, `\`))

	return trailing%2 == 1
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

	// /ipv6 nd keeps no comment: the launcher's ether1 entry is matched by interface.
	switch section {
	case "/ip address", "/ipv6 address", "/ip dhcp-client", "/ipv6 nd":
		return has("interface=ether1")
	}

	return false
}

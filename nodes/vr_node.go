package nodes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	VMInterfaceRegexp = regexp.MustCompile(`eth[1-9]\d*$`) // skipcq: GO-C4007
	imageTagRE        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

type VRNode struct {
	DefaultNode
	ScrapliPlatformName string
	ConfigDirName       string
	StartupCfgFName     string
	Credentials         *Credentials
}

func NewVRNode(n NodeOverwrites, creds *Credentials, scrapliPlatformName string) *VRNode {
	vr := &VRNode{}

	vr.DefaultNode = *NewDefaultNode(n)

	vr.Credentials = creds
	vr.ScrapliPlatformName = scrapliPlatformName

	vr.InterfaceMappedPrefix = "eth"
	vr.InterfaceOffset = 0
	vr.FirstDataIfIndex = 1
	vr.ConfigDirName = "config"
	vr.StartupCfgFName = "startup-config.cfg"

	return vr
}

// LinkApplyMode keeps vrnetlab-backed VM nodes on the conservative recreate
// path for apply link changes.
func (vr *VRNode) LinkApplyMode(ctx context.Context) LinkApplyMode {
	return vr.ImageLinkApplyMode(ctx, LinkApplyModeRecreate)
}

// Init stub function.
func (n *VRNode) Init(cfg *clabtypes.NodeConfig, opts ...NodeOption) error {
	return nil
}

// PreDeploy default function: create lab directory, generate certificates, generate startup config
// file.
func (n *VRNode) PreDeploy(_ context.Context, params *PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	_, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return err
	}
	return LoadStartupConfigFileVr(n, n.ConfigDirName, n.StartupCfgFName)
}

// AddEndpoint override version maps the endpoint name to an ethX-based name before adding it to the
// node endpoints. Returns an error if the mapping goes wrong.
func (vr *VRNode) AddEndpoint(e clablinks.Endpoint) error {
	endpointName := e.GetIfaceName()
	// Slightly modified check: if it doesn't match the VMInterfaceRegexp, pass it to
	// GetMappedInterfaceName. If it fails, then the interface name is wrong.
	if vr.InterfaceRegexp != nil && !(VMInterfaceRegexp.MatchString(endpointName)) {
		mappedName, err := vr.OverwriteNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf(
				"%q interface name %q could not be mapped to an ethX-based interface name: %w",
				vr.Cfg.ShortName,
				e.GetIfaceName(),
				err,
			)
		}
		log.Debugf(
			"Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)",
			endpointName,
			mappedName,
		)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}

	if e.GetNode() == nil {
		e.SetNode(vr)
	}

	vr.Endpoints = append(vr.Endpoints, e)
	return nil
}

// CheckInterfaceName checks interface names for generic VM-based nodes.
// Displays InterfaceHelp if the check fails for the expected VM interface regexp.
func (vr *VRNode) CheckInterfaceName() error {
	err := vr.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, ep := range vr.Endpoints {
		ifName := ep.GetIfaceName()
		if !VMInterfaceRegexp.MatchString(ifName) {
			return fmt.Errorf(
				"%q interface name %q does not match the required interface patterns: %q",
				vr.Cfg.ShortName,
				ifName,
				vr.InterfaceHelp,
			)
		}
	}

	return nil
}

// mgmtIPTokenRE builds a regexp matching the given management IP address as a
// standalone token, so that e.g. 172.20.20.2 does not also match 172.20.20.20.
// The address is bounded on both sides by any character that is not part of an
// IPv4/IPv6 literal (hex digit, dot or colon).
func mgmtIPTokenRE(ip string) *regexp.Regexp {
	const bound = `[^0-9A-Fa-f:.]`
	return regexp.MustCompile(`(?:^|` + bound + `)` + regexp.QuoteMeta(ip) + `(?:` + bound + `|$)`)
}

// addressAssignmentRE guards the mgmt-IP filter to lines that actually assign an
// address (interface `ip address` / `ipv4 address` / Junos `address x/y;` /
// RouterOS `add address=`), so we never drop unrelated lines that merely
// reference the mgmt IP (e.g. `snmp-server host`, `ntp server`, a BGP neighbor).
var addressAssignmentRE = regexp.MustCompile(`(?i)address`)

// mgmtConfigFilterPlatforms is the set of scrapli platform names whose config
// syntax we've verified against real `show running-config`-style output to
// confirm the management address is always a single leaf line containing the
// literal word "address" (case-insensitive) that FilterMgmtIPConfigLines's
// addressAssignmentRE guard requires: Cisco IOS/IOS-XE/IOS-XR/NX-OS/ASA,
// Arista EOS, Huawei VRP, Dell EMC OS10, IP Infusion OcNOS all use e.g.
// "ip address x.x.x.x ..." / "ipv4 address x.x.x.x" as a leaf statement under
// the interface; juniper_junos uses curly-brace hierarchy, handled brace-aware.
//
// Deliberately excluded: Aruba AOS-CX configures its mgmt interface with
// "ip static <addr>/<mask>", not "ip address" -- no "address" substring at all
// -- so the guard above would silently never match it (a no-op, not a
// corruption, but not the intended protection either). Add it once the filter
// also recognizes that syntax.
//
// Any platform not listed here is left untouched (no-op), so unverified kinds
// never regress.
var mgmtConfigFilterPlatforms = map[string]bool{
	"cisco_asa":        true,
	"cisco_ios":        true,
	"cisco_iosxe":      true,
	"cisco_iosxr":      true,
	"cisco_nxos":       true,
	"arista_eos":       true,
	"huawei_vrp":       true,
	"dell_emc":         true,
	"ipinfusion_ocnos": true,
	"juniper_junos":    true,
}

// FilterMgmtIPConfigLines removes the node's clab-assigned management IP
// address(es) from a saved device configuration, structurally.
//
// VM-based (vrnetlab) nodes have their management interface configured by the
// vrnetlab launcher from the container's *actual* IP on every boot. Persisting
// that address into the saved startup-config pins a now-fixed IP that is
// re-applied verbatim on the next deploy; if the container is later assigned a
// different management IP (e.g. a partial `--node-filter` deploy, or docker
// IPAM handing out a different address), the router comes up with mgmt on the
// stale IP while clab/DNS expect the new one -- "healthy" but unreachable. By
// dropping the launcher-managed address at save time we keep the launcher as the
// single source of truth for management addressing.
//
// Device CLIs are stateful/hierarchical, so we do not blindly delete matching
// lines (that would unbalance a Junos "address x/y { ... }" block or similar).
// Instead, for the line carrying the mgmt IP:
//   - if it opens a hierarchy block ("{"), the whole *balanced* block is removed
//     so braces and children stay consistent (Junos with address options);
//   - otherwise it is a leaf statement (IOS-XR "ipv4 address x", IOS "ip address
//     x", Junos "address x/y;", RouterOS "add address=x/y ...") and only that
//     line is removed, leaving the enclosing interface/context header intact so
//     the launcher can re-add the address on the next boot.
//
// This is vendor-agnostic: it keys off the assigned IP rather than each vendor's
// management-interface name, while still respecting config structure.
func FilterMgmtIPConfigLines(config, mgmtV4, mgmtV6 string) string {
	var res []*regexp.Regexp
	for _, ip := range []string{mgmtV4, mgmtV6} {
		if ip != "" {
			res = append(res, mgmtIPTokenRE(ip))
		}
	}
	if len(res) == 0 {
		return config
	}

	// A line is a removal candidate only if it both carries a mgmt IP token and
	// looks like an address assignment (guard against unrelated references).
	matches := func(s string) bool {
		if !addressAssignmentRE.MatchString(s) {
			return false
		}
		for _, re := range res {
			if re.MatchString(s) {
				return true
			}
		}
		return false
	}

	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		line := lines[i]
		if !matches(line) {
			out = append(out, line)
			i++
			continue
		}

		// The mgmt IP is on this line. Decide leaf vs. block by net brace depth.
		depth := strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 {
			i++ // leaf statement: drop just this line
			continue
		}
		// Block opener: consume lines until the braces it opened are balanced.
		for i++; i < len(lines) && depth > 0; i++ {
			depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
		}
	}
	return strings.Join(out, "\n")
}

func (n *VRNode) SaveConfig(_ context.Context) (*SaveConfigResult, error) {
	config, err := clabnetconf.GetConfig(n.Cfg.LongName,
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
		n.ScrapliPlatformName,
	)
	if err != nil {
		return nil, err
	}

	// Do not persist the launcher-managed management address; it is re-applied
	// from the container's actual IP on every boot and would otherwise pin a
	// stale mgmt IP on redeploy (see FilterMgmtIPConfigLines). Only for platforms
	// whose config syntax we handle safely; others are left untouched.
	if mgmtConfigFilterPlatforms[n.ScrapliPlatformName] {
		config = FilterMgmtIPConfigLines(config, n.Cfg.MgmtIPv4Address, n.Cfg.MgmtIPv6Address)
	}

	// Save config to mounted labdir startup config path
	configPath := filepath.Join(n.Cfg.LabDir, n.ConfigDirName, n.StartupCfgFName)
	err = os.WriteFile(
		configPath,
		[]byte(config),
		clabconstants.PermissionsOpen,
	) // skipcq: GO-S2306
	if err != nil {
		return nil, fmt.Errorf(
			"failed to write config by %s path from %s container: %v",
			configPath,
			n.Cfg.ShortName,
			err,
		)
	}
	log.Info("Saved configuration to path", "nodeName", n.Cfg.ShortName, "path", configPath)

	return &SaveConfigResult{
		ConfigPath: configPath,
	}, nil
}

// PreStop prepares vrnetlab-specific state before DefaultNode.Stop parks interfaces and stops the
// container.
func (vr *VRNode) PreStop(ctx context.Context) error {
	preStopPrepareVrnetlabQcowAlias(ctx, &vr.DefaultNode)
	return nil
}

func preStopPrepareVrnetlabQcowAlias(ctx context.Context, d *DefaultNode) {
	aliasName, ok := vrnetlabQcowAliasName(d.Config().Image)
	if !ok {
		log.Debugf(
			"node %q pre-stop vrnetlab qcow alias skipped: unable to infer tag from image %q",
			d.Config().ShortName,
			d.Config().Image,
		)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Some vrnetlab nodes rename the original versioned qcow image after first boot and fail on
	// subsequent starts when they try to rediscover a versioned qcow filename. If there is exactly
	// one non-overlay qcow file in / and our alias is absent, create a hardlink alias based on the
	// image tag.
	cmd := fmt.Sprintf(
		`alias="/%s"; `+
			`[ -e "$alias" ] && exit 0; `+
			`src=""; `+
			`if [ -f /sros.qcow2 ] && [ "/sros.qcow2" != "$alias" ]; then `+
			`src="/sros.qcow2"; `+
			`else `+
			`set -- /*.qcow2; `+
			`if [ "$1" != "/*.qcow2" ]; then `+
			`for f in "$@"; do `+
			`[ "$f" = "$alias" ] && continue; `+
			`base="${f##*/}"; `+
			`case "$base" in *overlay*.qcow2) continue ;; esac; `+
			`if [ -n "$src" ]; then src=""; break; fi; `+
			`src="$f"; `+
			`done; `+
			`fi; `+
			`fi; `+
			`[ -n "$src" ] || exit 0; `+
			`ln "$src" "$alias"`,
		aliasName,
	)

	execCmd := clabexec.NewExecCmdFromSlice([]string{"sh", "-lc", cmd})
	res, err := d.RunExec(ctx, execCmd)
	if err != nil {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias preparation failed: %v",
			d.Config().ShortName,
			err,
		)
		return
	}

	if res != nil && res.ReturnCode != 0 {
		log.Warnf(
			"node %q pre-stop vrnetlab qcow alias prep returned code %d (stderr: %s)",
			d.Config().ShortName,
			res.ReturnCode,
			res.Stderr,
		)
	}
}

func vrnetlabQcowAliasName(image string) (string, bool) {
	tag, ok := imageTag(image)
	if !ok {
		return "", false
	}

	return "clab-" + tag + ".qcow2", true
}

func imageTag(image string) (string, bool) {
	if at := strings.LastIndex(image, "@"); at != -1 {
		image = image[:at]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 || lastColon < lastSlash {
		return "", false
	}

	tag := image[lastColon+1:]
	if tag == "" {
		return "", false
	}

	if !imageTagRE.MatchString(tag) {
		return "", false
	}

	return tag, true
}

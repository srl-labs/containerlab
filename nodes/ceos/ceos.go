// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	ifWaitScriptContainerPath = "/mnt/flash/if-wait.sh"
	generateable              = true
	generateIfFormat          = "eth%d"

	scrapliPlatformName = "arista_eos"
	NapalmPlatformName  = "eos"

	tlsKeyFile  = "node.key"
	tlsCertFile = "node.crt"
	tlsCAFile   = "ca.crt"

	// MGMT_INTF selects the container netdev cEOS maps to the management
	// interface; MAPETH0 is a legacy companion flag kept in the default env.
	mgmtIntfEnvVar = "MGMT_INTF"
	mapEth0EnvVar  = "MAPETH0"
	// default management netdev, mapped to Management0 by cEOS.
	defaultMgmtIntfNetdev = "eth0"

	networkModeNone = "none"
)

var (
	KindNames = []string{"ceos", "arista_ceos"}
	// defined env vars for the ceos.
	ceosEnv = map[string]string{
		"CEOS":                                "1",
		"EOS_PLATFORM":                        "ceoslab",
		"container":                           "docker",
		"ETBA":                                "1",
		"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT": "1",
		"INTFTYPE":                            "eth",
		mapEth0EnvVar:                         "1",
		mgmtIntfEnvVar:                        defaultMgmtIntfNetdev,
	}

	//go:embed ceos.cfg
	cfgTemplate string

	saveCmd = "Cli -p 15 -c wr"

	defaultCredentials = clabnodes.NewCredentials("admin", "admin")

	// maN management netdev (e.g. ma1 -> Management1).
	mgmtIntfRegexp = regexp.MustCompile(`^ma([0-9]+)$`)
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformAttrs := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformAttrs,
	)

	r.Register(KindNames, func() clabnodes.Node {
		return new(ceos)
	}, nrea)
}

type ceos struct {
	clabnodes.DefaultNode
}

// intfMap represents interface mapping config file.
type intfMap struct {
	ManagementIntf struct {
		Eth0 string `json:"eth0"`
	} `json:"ManagementIntf"`
	EthernetIntf struct {
		eth map[string]string
	} `json:"EthernetIntf"`
}

func (n *ceos) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg

	n.StopSignal = clabtypes.SIGRTMIN3

	for _, o := range opts {
		o(n)
	}

	n.Cfg.Env = clabutils.MergeStringMaps(ceosEnv, n.Cfg.Env)

	// containerlab only special-cases a maN management netdev; anything else is
	// passed through to cEOS unchanged. Warn about values that cEOS would map to
	// an unexpected interface (e.g. MGMT_INTF=eth1 hijacks a data interface).
	mgmtNetdev := n.mgmtNetdev()
	if mgmtNetdev != defaultMgmtIntfNetdev && !n.mgmtRemapped() {
		log.Warnf(
			"node %q: MGMT_INTF=%q is not handled by containerlab; only %q (default) or maN are supported, cEOS may use an unexpected interface for management",
			n.Cfg.ShortName, mgmtNetdev, defaultMgmtIntfNetdev,
		)
	}

	// MGMT_INTF alone selects the management interface, so drop the legacy
	// MAPETH0 flag once the mgmt netdev is remapped to maN.
	if n.mgmtRemapped() {
		delete(n.Cfg.Env, mapEth0EnvVar)
	}

	// the node.Cmd should be aligned with the environment.
	// prepending original Cmd with if-wait.sh script to make sure that interfaces are available
	// before init process starts
	var envSb strings.Builder
	envSb.WriteString("bash -c '" + ifWaitScriptContainerPath + " ; ")

	// The runtime always names the mgmt veth eth0. When MGMT_INTF remaps the mgmt
	// interface (e.g. ma1 -> Management1) rename eth0 accordingly before init.
	// Only do this when attached to the runtime mgmt network: with host or
	// container network modes eth0 belongs to a foreign namespace, and with
	// network-mode none there is no eth0 (the netdev comes from a link).
	if n.mgmtRemapped() && n.mgmtViaRuntimeNetwork() {
		envSb.WriteString(renameMgmtNetdevCmd(mgmtNetdev) + " ; ")
	}

	envSb.WriteString("exec /sbin/init ")
	for k, v := range n.Cfg.Env {
		envSb.WriteString("systemd.setenv=\"" + k + "=" + v + "\" ")
	}
	envSb.WriteString("'")

	n.Cfg.Cmd = envSb.String()
	hwa, err := clabutils.GenMac("00:1c:73")
	if err != nil {
		return err
	}
	n.Cfg.MacAddress = hwa.String()

	// create TLS certificates for the node by default.
	// The cert, key and CA files are mounted into the container
	// and can be validated with `show management security ssl certificate`.
	n.Cfg.Certificate.Issue = clabutils.Pointer(true)

	// mount config dir
	cfgPath := filepath.Join(n.Cfg.LabDir, "flash")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprintf("%s:/mnt/flash/", cfgPath))

	if *n.Cfg.Certificate.Issue {
		keyPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsKeyFile)
		certPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsCertFile)
		caPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsCAFile)

		n.Cfg.Binds = append(n.Cfg.Binds,
			fmt.Sprintf("%s:/persist/secure/ssl/keys/%s", keyPath, tlsKeyFile),
			fmt.Sprintf("%s:/persist/secure/ssl/certs/%s", certPath, tlsCertFile),
			fmt.Sprintf("%s:/persist/secure/ssl/certs/%s", caPath, tlsCAFile),
		)
	}

	return nil
}

func (n *ceos) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	clabutils.CreateDirectory(n.Cfg.LabDir, clabconstants.PermissionsOpen)
	if *n.Cfg.Certificate.Issue {
		certificate, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
		if err != nil {
			return err
		}

		caCertificate, err := params.Cert.LoadCaCert()
		if err != nil {
			return err
		}

		n.Config().TLSCert = string(certificate.Cert)
		n.Config().TLSKey = string(certificate.Key)
		n.Config().TLSAnchor = string(caCertificate.Cert)
	}
	return n.createCEOSFiles(ctx)
}

func (n *ceos) PostDeploy(ctx context.Context, _ *clabnodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Arista cEOS '%s' node", n.Cfg.ShortName)
	return n.ceosPostDeploy(ctx)
}

func (n *ceos) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	cmd, _ := clabexec.NewExecCmdFromString(saveCmd)
	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute cmd: %v", n.Cfg.ShortName, err)
	}

	if execResult.GetStdErrString() != "" {
		return nil, fmt.Errorf("%s errors: %s", n.Cfg.ShortName, execResult.GetStdErrString())
	}

	cfgPath := filepath.Join(n.Cfg.LabDir, "flash", "startup-config")
	log.Infof("saved cEOS configuration from %s node to %s\n", n.Cfg.ShortName, cfgPath)

	return &clabnodes.SaveConfigResult{
		ConfigPath: cfgPath,
	}, nil
}

func (n *ceos) createCEOSFiles(ctx context.Context) error {
	nodeCfg := n.Config()
	// generate config directory
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, "flash"),
		clabconstants.PermissionsOpen)
	cfg := filepath.Join(n.Cfg.LabDir, "flash", "startup-config")
	nodeCfg.ResStartupConfig = cfg

	// set mgmt ipv4 gateway as it is already known by now
	// since the container network has been created before we launch nodes
	// and mgmt gateway can be used in ceos.Cfg template to configure default route for mgmt.
	// Skip it when the mgmt interface is wired manually via a link, as the node
	// is not attached to the runtime mgmt network.
	if !n.mgmtWiredViaLink() {
		nodeCfg.MgmtIPv4Gateway = n.Runtime.Mgmt().IPv4Gw
		nodeCfg.MgmtIPv6Gateway = n.Runtime.Mgmt().IPv6Gw
	}

	// set the mgmt interface name for the node
	err := setMgmtInterface(nodeCfg)
	if err != nil {
		return err
	}

	// use startup config file provided by a user
	// make copy of template to prevent provided startup config from mutating shared package
	// template value
	currentCfgTemplate := cfgTemplate
	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		currentCfgTemplate = string(c)
	}

	err = n.GenerateConfig(nodeCfg.ResStartupConfig, currentCfgTemplate)
	if err != nil {
		return err
	}

	// if extras have been provided copy these into the flash directory
	if nodeCfg.Extras != nil && len(nodeCfg.Extras.CeosCopyToFlash) != 0 {
		extras := nodeCfg.Extras.CeosCopyToFlash
		flash := filepath.Join(nodeCfg.LabDir, "flash")

		for _, extrapath := range extras {
			basename := filepath.Base(extrapath)
			dest := filepath.Join(flash, basename)

			topoDir := filepath.Dir(
				filepath.Dir(nodeCfg.LabDir),
			) // topo dir is needed to resolve extrapaths
			if err := clabutils.CopyFile(ctx,
				clabutils.ResolvePath(extrapath, topoDir), dest,
				clabconstants.PermissionsFileDefault); err != nil {
				return fmt.Errorf("extras: copy-to-flash %s -> %s failed %v", extrapath, dest, err)
			}
		}
	}

	// sysmac is a system mac that is +1 to Ma0 mac
	m, err := net.ParseMAC(nodeCfg.MacAddress)
	if err != nil {
		return err
	}
	m[5]++

	sysMacPath := path.Join(nodeCfg.LabDir, "flash", "system_mac_address")

	if !clabutils.FileExists(sysMacPath) {
		err = clabutils.CreateFile(sysMacPath, m.String())
	}

	if err != nil {
		return err
	}

	// adding if-wait.sh script to flash dir
	ifScriptP := path.Join(nodeCfg.LabDir, "flash", "if-wait.sh")
	if err := clabutils.CreateFile(ifScriptP, clabutils.IfWaitScript); err != nil {
		return fmt.Errorf("failed to write if-wait.sh: %w", err)
	}
	os.Chmod(ifScriptP, clabconstants.PermissionsOpen) // skipcq: GSC-G302

	if *n.Cfg.Certificate.Issue {
		err = n.createCEOSCertificates()
	}

	return err
}

// Func that Places the Certificates in the right place and format.
func (n *ceos) createCEOSCertificates() error {
	if *n.Cfg.Certificate.Issue {
		clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, "ssl"), clabconstants.PermissionsOpen)

		keyPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsKeyFile)
		if err := clabutils.CreateFile(keyPath, n.Config().TLSKey); err != nil {
			return err
		}

		certPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsCertFile)
		if err := clabutils.CreateFile(certPath, n.Config().TLSCert); err != nil {
			return err
		}

		caPath := filepath.Join(n.Cfg.LabDir, "ssl", tlsCAFile)
		if err := clabutils.CreateFile(caPath, n.Config().TLSAnchor); err != nil {
			return err
		}
	}
	return nil
}

func setMgmtInterface(node *clabtypes.NodeConfig) error {
	mgmtNetdev := node.Env[mgmtIntfEnvVar]
	if mgmtNetdev == "" {
		mgmtNetdev = defaultMgmtIntfNetdev
	}

	// a maN netdev (e.g. ma1) is exposed by cEOS as ManagementN
	if m := mgmtIntfRegexp.FindStringSubmatch(mgmtNetdev); m != nil {
		node.MgmtIntf = "Management" + m[1]
		log.Debugf("Management interface for '%s' node is set to %s.", node.ShortName, node.MgmtIntf)
		return nil
	}

	// otherwise eth0 maps to Management0, unless an EosIntfMapping.json overrides it
	mgmtInterface := "Management0"
	for _, bindelement := range node.Binds {
		if !strings.Contains(bindelement, "EosIntfMapping.json") {
			continue
		}

		bindsplit := strings.Split(bindelement, ":")
		if len(bindsplit) < 2 {
			return fmt.Errorf("malformed bind instruction: %s", bindelement)
		}

		var m []byte // byte representation of a map file
		m, err := os.ReadFile(bindsplit[0])
		if err != nil {
			return err
		}

		// Reset management interface if defined in the intfMapping file
		var intfMappingJson intfMap
		err = json.Unmarshal(m, &intfMappingJson)
		if err != nil {
			log.Debugf(
				"Management interface could not be read from intfMapping file for '%s' node.",
				node.ShortName,
			)
			return err
		}
		mgmtInterface = intfMappingJson.ManagementIntf.Eth0
	}
	log.Debugf("Management interface for '%s' node is set to %s.", node.ShortName, mgmtInterface)
	node.MgmtIntf = mgmtInterface

	return nil
}

// mgmtNetdev returns the container netdev cEOS maps to the management interface,
// as set by MGMT_INTF (default eth0).
func (n *ceos) mgmtNetdev() string {
	if d := n.Cfg.Env[mgmtIntfEnvVar]; d != "" {
		return d
	}
	return defaultMgmtIntfNetdev
}

// mgmtRemapped reports whether the management netdev is remapped to a maN netdev
// (e.g. ma1 -> Management1). This is the single switch that enables the
// management interface handling; any other MGMT_INTF value is left to cEOS.
func (n *ceos) mgmtRemapped() bool {
	return mgmtIntfRegexp.MatchString(n.mgmtNetdev())
}

// mgmtWiredViaLink reports whether the management interface is wired manually via
// a topology link: the netdev is remapped to maN and the node opts out of the
// runtime mgmt network with network-mode: none.
func (n *ceos) mgmtWiredViaLink() bool {
	return n.mgmtRemapped() && n.Cfg.NetworkMode == networkModeNone
}

// mgmtViaRuntimeNetwork reports whether the node uses the runtime's management
// network (the default mode), where eth0 is a runtime-created veth that can be
// safely renamed. Explicit modes (none, host, container:<name>) either have no
// eth0 or share a foreign namespace whose eth0 must not be renamed.
func (n *ceos) mgmtViaRuntimeNetwork() bool {
	return n.Cfg.NetworkMode == ""
}

// renameMgmtNetdevCmd returns a shell snippet renaming eth0 to the netdev cEOS
// expects (e.g. ma1 -> Management1). The existence guard is defence-in-depth.
func renameMgmtNetdevCmd(netdev string) string {
	return fmt.Sprintf(
		"if [ -e /sys/class/net/%[1]s ]; then "+
			"ip link set %[1]s down && ip link set %[1]s name %[2]s && ip link set %[2]s up; fi",
		defaultMgmtIntfNetdev, netdev,
	)
}

// mgmtAddressFromConfig returns the runtime-assigned management IPv4 and IPv6
// addresses in CIDR notation, or empty strings when none were assigned.
func mgmtAddressFromConfig(nodeCfg *clabtypes.NodeConfig) (v4, v6 string) {
	if nodeCfg.MgmtIPv4Address != "" {
		v4 = fmt.Sprintf("%s/%d", nodeCfg.MgmtIPv4Address, nodeCfg.MgmtIPv4PrefixLength)
	}
	if nodeCfg.MgmtIPv6Address != "" {
		v6 = fmt.Sprintf("%s/%d", nodeCfg.MgmtIPv6Address, nodeCfg.MgmtIPv6PrefixLength)
	}
	return v4, v6
}

// postDeployConfig builds the EOS configuration pushed after deploy: the
// management interface addressing (when an address is known) followed by the
// data interface addressing. It returns nil when there is nothing to configure,
// for example a manually wired management interface left to its startup-config.
func (n *ceos) postDeployConfig() []string {
	nodeCfg := n.Config()

	var cfgs []string

	// mgmt address comes from the runtime mgmt network; when wired manually
	// (network-mode: none) fall back to the maN link endpoint address, if any.
	mgmtV4, mgmtV6 := mgmtAddressFromConfig(nodeCfg)
	if n.mgmtRemapped() && mgmtV4 == "" && mgmtV6 == "" {
		for _, e := range n.Endpoints {
			if e.GetIfaceName() != n.mgmtNetdev() {
				continue
			}
			if v4 := e.GetIPv4Addr(); v4.IsValid() {
				mgmtV4 = v4.String()
			}
			if v6 := e.GetIPv6Addr(); v6.IsValid() {
				mgmtV6 = v6.String()
			}
		}
	}

	// the legacy path always (re)configures the mgmt interface; the remapped path
	// only does so when it has an address, leaving a startup-config address intact.
	if !n.mgmtRemapped() || mgmtV4 != "" || mgmtV6 != "" {
		cfgs = append(cfgs,
			"interface "+nodeCfg.MgmtIntf,
			"no ip address",
			"no ipv6 address",
		)
		if mgmtV4 != "" {
			cfgs = append(cfgs, "ip address "+mgmtV4)
		}
		if mgmtV6 != "" {
			cfgs = append(cfgs, "ipv6 address "+mgmtV6)
		}
	}

	// skip the mgmt interface in the data loop. When remapped it is a maN kernel
	// netdev; otherwise the EOS name is used, which never matches a kernel
	// endpoint name (legacy behaviour, effectively no skip).
	skipIface := nodeCfg.MgmtIntf
	if n.mgmtRemapped() {
		skipIface = n.mgmtNetdev()
	}

	// configure data interfaces
	for _, e := range n.Endpoints {
		ifName := e.GetIfaceName()
		if ifName == skipIface {
			continue
		}

		v4 := e.GetIPv4Addr()
		v6 := e.GetIPv6Addr()

		if !v4.IsValid() && !v6.IsValid() {
			continue
		}

		cfgs = append(cfgs, "interface "+ifName)
		cfgs = append(cfgs, "no switchport")
		cfgs = append(cfgs, "no ip address")
		cfgs = append(cfgs, "no ipv6 address")

		if v4.IsValid() {
			cfgs = append(cfgs, fmt.Sprintf("ip address %s", v4.String()))
		}
		if v6.IsValid() {
			cfgs = append(cfgs, fmt.Sprintf("ipv6 address %s", v6.String()))
		}
	}

	return cfgs
}

// ceosPostDeploy runs postdeploy actions which are required for ceos nodes.
func (n *ceos) ceosPostDeploy(_ context.Context) error {
	cfgs := n.postDeployConfig()

	// nothing to configure (e.g. manually wired mgmt without addresses) - leave
	// the running/startup config untouched and skip opening a CLI session.
	if len(cfgs) == 0 {
		return nil
	}

	nodeCfg := n.Config()
	d, err := clabutils.SpawnCLIviaExec("arista_eos", nodeCfg.LongName, n.Runtime.GetName())
	if err != nil {
		return err
	}

	defer d.Close()

	// add save to startup cmd
	cfgs = append(cfgs, "wr")

	log.Debugf("cEOS PostDeploy configuration for node %s: %v", n.Cfg.ShortName, cfgs)

	resp, err := d.SendConfigs(cfgs)
	if err != nil {
		return err
	} else if resp.Failed != nil {
		return errors.New("failed CLI configuration")
	}

	return err
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *ceos) CheckInterfaceName() error {
	// allow eth and et interfaces
	// https://regex101.com/r/umQW5Z/2
	ifRe := regexp.MustCompile(`eth[1-9][\w.]*$|et[1-9][\w.]*$`)
	mgmtNetdev := n.mgmtNetdev()

	for _, e := range n.Endpoints {
		ifName := e.GetIfaceName()
		if ifRe.MatchString(ifName) {
			continue
		}

		// a maN interface may be wired manually, but only with network-mode: none
		// and only the netdev MGMT_INTF points at (e.g. ma1 -> Management1)
		if mgmtIntfRegexp.MatchString(ifName) {
			switch {
			case n.Cfg.NetworkMode != networkModeNone:
				return fmt.Errorf(
					"arista cEOS node %q wires management interface %q via a link, which requires network-mode: none",
					n.Cfg.ShortName,
					ifName,
				)
			case ifName != mgmtNetdev:
				return fmt.Errorf(
					"arista cEOS node %q wires management interface %q but its MGMT_INTF env is %q; the link endpoint name must match MGMT_INTF (e.g. set MGMT_INTF=%s)",
					n.Cfg.ShortName,
					ifName,
					mgmtNetdev,
					ifName,
				)
			default:
				continue
			}
		}

		return fmt.Errorf(
			"arista cEOS node %q has an interface named %q which doesn't match the required pattern. Interfaces should be named as ethX or etX, where X consists of alpanumerical characters",
			n.Cfg.ShortName,
			ifName,
		)
	}

	return nil
}

// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/beevik/etree"
	"github.com/brunoga/deep"
	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/opoptions"

	"github.com/scrapli/scrapligo/response"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
)

const (
	SrosDefaultType = "SR-1" // default sros node type

	readyTimeout = time.Minute * 1 // max wait time for node to boot

	generateable     = true
	generateIfFormat = "%d/%d/%d"

	slotAName = "A"
	slotBName = "B"

	standaloneSlotName = slotAName

	retryTimer = 1 * time.Second

	scrapliPlatformName          = "nokia_sros"
	scrapliPlatformNameClassic   = "nokia_sros_classic"
	configCf3                    = "config/cf3"
	configCf2                    = "config/cf2"
	configCf1                    = "config/cf1"
	startupCfgName               = "config.cfg"
	tlsKeyFile                   = "node.key"
	tlsCertFile                  = "node.crt"
	tlsCertProfileName           = "clab-grpc-certs"
	envNokiaSrosSlot             = "NOKIA_SROS_SLOT"
	envNokiaSrosChassis          = "NOKIA_SROS_CHASSIS"
	envNokiaSrosSystemBaseMac    = "NOKIA_SROS_SYSTEM_BASE_MAC"
	envNokiaSrosCard             = "NOKIA_SROS_CARD"
	envNokiaSrosSFM              = "NOKIA_SROS_SFM"
	envNokiaSrosXIOM             = "NOKIA_SROS_XIOM"
	envNokiaSrosMDA              = "NOKIA_SROS_MDA"
	envDisableComponentConfigGen = "CLAB_SROS_DISABLE_COMPONENT_CONFIG"
	envSrosConfigMode            = "CLAB_SROS_CONFIG_MODE"

	defaultSrosPowerType       = "dc"
	defaultSrosPowerModuleType = "ps-a-dc-6000"

	srosMinorError     = "MINOR:"
	srosCriticalError  = "CRITICAL:"
	srosRejectedCfgMsg = "Configuration load failed - using default configuration"
)

var (

	//go:embed configs/sros_config_sros25.go.tpl
	cfgTplSROS25 string

	//go:embed configs/sros_config_classic.go.tpl
	cfgTplClassic string

	//go:embed configs/ixr/ixr_config_classic.go.tpl
	cfgTplClassicIxr string

	//go:embed configs/sar/sar_config_classic.go.tpl
	cfgTplClassicSar string

	kindNames  = []string{"nokia_srsim"}
	srosSysctl = map[string]string{
		"net.ipv4.ip_forward":                "0",
		"net.ipv6.conf.all.disable_ipv6":     "0",
		"net.ipv6.conf.default.disable_ipv6": "0",
		"net.ipv6.conf.all.accept_dad":       "0",
		"net.ipv6.conf.default.accept_dad":   "0",
		"net.ipv6.conf.all.autoconf":         "0",
		"net.ipv6.conf.default.autoconf":     "0",
		"net.ipv4.conf.all.rp_filter":        "0",
		"net.ipv6.conf.all.accept_ra":        "0",
		"net.ipv6.conf.default.accept_ra":    "0",
		"net.ipv4.conf.default.rp_filter":    "0",
	}
	defaultCredentials = clabnodes.NewCredentials("admin", "NokiaSros1!")

	srosEnv = map[string]string{
		"SRSIM":                   "1",
		envNokiaSrosChassis:       SrosDefaultType,     // filler to be overridden
		envNokiaSrosSystemBaseMac: "fa:ac:ff:ff:10:00", // filler to be overridden
		envNokiaSrosSlot:          slotAName,           // filler to be overridden
		envSrosConfigMode:         "model-driven",      // Default
	}

	readyCmdCpm  = `/usr/bin/pgrep ^cpm$`
	readyCmdBoth = `/usr/bin/pgrep ^both$`
	readyCmdIom  = `/usr/bin/pgrep ^iom$`

	requiredKernelVersion = &clabutils.KernelVersion{
		Major:    5,
		Minor:    5,
		Revision: 0,
	}
	// Internal directories inside SR-SIM container.
	cf1Dir = "/home/sros/flash1"
	cf2Dir = "/home/sros/flash2"
	cf3Dir = "/home/sros/flash3" // Where the running config will be stored
	licDir = "/nokia/license"

	InterfaceRegexp = regexp.MustCompile(
		`^(?P<card>\d+)/(?:x(?P<xiom>\d+)/)?(?P<mda>\d+)(?:/c(?P<connector>\d+))?/(?P<port>\d+)$`,
	)
	MappedInterfaceRegexp = regexp.MustCompile(
		`^(?:e(?P<card>\d+)-(?:x(?P<xiom>\d+)-)?(?P<mda>\d+)(?:-c(?P<connector>\d+))?-(?P<port>\d+)|eth(?P<mgmtPort>\d+))$`,
	)
	InterfaceHelp = `The format of the interface name need to be one of:
	  Regular SR OS interface names, that is:
	  1/2/3       -> card 1, mda 2, port 3
      1/2/c3/4    -> card 1, mda 2, connector 3, port 4
      1/x2/3/4    -> card 1, xiom 2, mda 3, port 4
      1/x2/3/c4/5 -> card 1, xiom 2, mda 3, connector 4, port 5
	  The mapped Linux interface names, that is:
      e1-2-3       -> card 1, mda 2, port 3
      e1-2-c3-4    -> card 1, mda 2, connector 3, port 4
      e1-x2-3-4    -> card 1, xiom 2, mda 3, port 4
      e1-x2-3-c4-5 -> card 1, xiom 2, mda 3, connector 4, port 5
	  eth[0-9], for management interfaces of CPM-A/CPM-B or for fabric interfaces`
	// Auxiliary regexps for IXR/SAR detection.
	sarRegexp   = regexp.MustCompile(`(?i)\bsar-`)
	sarHmRegexp = regexp.MustCompile(`(?i)\b(sar-hm|sar-hmc)\b`)

	ixrRegexp = regexp.MustCompile(`(?i)\bixr-`)
)

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	generateNodeAttributes := clabnodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformOpts := &clabnodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}
	nrea := clabnodes.NewNodeRegistryEntryAttributes(
		defaultCredentials,
		generateNodeAttributes,
		platformOpts,
	)

	r.Register(kindNames, func() clabnodes.Node {
		return new(sros)
	}, nrea)
}

// sros SR-SIM Kind structure.
type sros struct {
	clabnodes.DefaultNode
	// startup-config passed as a path to a file with CLI instructions will be read into this byte
	// slice
	startupCliCfg []byte

	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey

	// software version SR OS x node runs
	swVersion      *SrosVersion
	componentNodes []clabnodes.Node

	// in distributed mode we rename the Cfg.LongName and Cfg.ShortName and Cfg.Fqdn attributes when
	// deploying. e.g. inspect is either called after deploy or independently. Hence we need to
	// differentiate if we need to perform the
	// component cpm based rename or not. This field indicates just that
	renameDone bool
	// rootComponents stores the OG components from root node for dist setups
	// ..allows children of the distributed root node to access root components (ie. for cfg gen)
	rootComponents []*clabtypes.Component
	// store the longname with cpm suffix
	cpmContainerName string
	// for component nodes, store base nodes
	baseShortName string
	baseLongName  string

	preDeployParams *clabnodes.PreDeployParams
}

// Init Function for SR-SIM kind.
func (n *sros) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.SSSE3 = true
	n.HostRequirements.MinVCPU = 4
	n.HostRequirements.MinVCPUFailAction = clabtypes.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 4
	n.HostRequirements.MinAvailMemoryGbFailAction = clabtypes.FailBehaviourLog

	n.LicensePolicy = clabtypes.LicensePolicyRequired

	n.Cfg = cfg

	n.InterfaceHelp = InterfaceHelp
	n.InterfaceRegexp = InterfaceRegexp

	for _, o := range opts {
		o(n)
	}

	// if user was not initialized to a value, use root
	if n.Cfg.User == "" {
		n.Cfg.User = "0:0"
	}

	maps.Copy(n.Cfg.Sysctls, srosSysctl)

	// make sure we always have uppercase slot definition
	for _, c := range n.Cfg.Components {
		c.Slot = strings.ToUpper(c.Slot)
	}
	// Merge Environment
	if n.Cfg.NodeType == "" {
		n.Cfg.NodeType = SrosDefaultType
	}
	srosEnv[envNokiaSrosChassis] = n.Cfg.NodeType // Override NodeType var with existing env

	mac := genMac(n.Cfg)
	srosEnv[envNokiaSrosSystemBaseMac] = mac
	// Ensure n.Cfg.Certificate.Issue is not nil
	if n.Cfg.Certificate.Issue == nil {
		n.Cfg.Certificate.Issue = clabutils.Pointer(false)
	}
	if n.isStandaloneNode() {
		log.Debugf("%q is standalone node. %v", n.Cfg.ShortName, len(n.Cfg.Components))

		vars, err := n.setupStandaloneComponents()
		if err != nil {
			return err
		}

		// n.Cfg.Env overrides component vars, overrides default srosEnv
		n.Cfg.Env = clabutils.MergeStringMaps(srosEnv, vars, n.Cfg.Env)
		log.Debug("Merged env file", "env", fmt.Sprintf("%+v", n.Cfg.Env), "node", n.Cfg.ShortName)
	} else {
		log.Debugf("%q is distributed node. %v", n.Cfg.ShortName, len(n.Cfg.Components))

		n.Cfg.Env = clabutils.MergeStringMaps(srosEnv, n.Cfg.Env)
		log.Debug("Merged env file", "env", fmt.Sprintf("%+v", n.Cfg.Env), "node", n.Cfg.ShortName)

		err := n.setupComponentNodes()
		if err != nil {
			return err
		}
	}

	return nil
}

// integrated -> pull component info from slot A components.
func (n *sros) setupStandaloneComponents() (map[string]string, error) {
	vars := map[string]string{}

	if len(n.Cfg.Components) == 0 {
		return nil, nil
	}

	slotA := n.Cfg.Components[0]

	slotName := strings.ToUpper(strings.TrimSpace(slotA.Slot))
	// single undefined slot is implicitly set to A
	if slotName == "" {
		slotName = standaloneSlotName
	}

	if slotName != standaloneSlotName {
		return nil, fmt.Errorf(
			"expected no slot, or slot %q for components of standalone SR-SIM node: %q",
			standaloneSlotName,
			n.Cfg.ShortName,
		)
	}

	if slotA.Type != "" {
		vars[envNokiaSrosCard] = slotA.Type
	}

	if len(slotA.MDA) > 0 {
		for _, m := range slotA.MDA {
			key := fmt.Sprintf("%s_%d", envNokiaSrosMDA, m.Slot)
			vars[key] = m.Type
		}
	}

	maps.Copy(vars, slotA.Env)

	return vars, nil
}

// Pre Deploy func for SR-SIM kind.
func (n *sros) PreDeploy(ctx context.Context, params *clabnodes.PreDeployParams) error {
	log.Debug("Running pre-deploy")

	if err := n.verifyNokiaSrsimImage(ctx); err != nil {
		return err
	}

	// store the preDeployParams
	n.preDeployParams = params

	n.InterfaceHelp = InterfaceHelp

	// store provided pubkeys
	n.sshPubKeys = params.SSHPubKeys

	// Create files/dir structure for standalone nodes or distributed CPM nodes
	if n.isStandaloneNode() || (n.isDistributedCardNode() && n.isCPM("")) {
		// generate the certificate
		if *n.Cfg.Certificate.Issue {
			n.Cfg.Certificate.SANs = append(n.Cfg.Certificate.SANs, n.baseShortName, n.baseLongName)
			certificate, err := n.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
			if err != nil {
				return err
			}

			// set the certificate data
			n.Config().TLSCert = string(certificate.Cert)
			n.Config().TLSKey = string(certificate.Key)
		}
		clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot]),
			clabconstants.PermissionsOpen)
		slot := n.Cfg.Env[envNokiaSrosSlot]
		if slot == "" {
			return fmt.Errorf("fail to init node because Env var %q is set to %q",
				envNokiaSrosSlot, n.Cfg.Env[envNokiaSrosSlot])
		}
		// add the config specific mounts
		cf1Path := filepath.Join(n.Cfg.LabDir, slot, configCf1)
		cf2Path := filepath.Join(n.Cfg.LabDir, slot, configCf2)
		cf3Path := filepath.Join(n.Cfg.LabDir, slot, configCf3)
		n.Cfg.Binds = append(n.Cfg.Binds,
			fmt.Sprint(cf1Path, ":", cf1Dir, "/:rw"),
			fmt.Sprint(cf2Path, ":", cf2Dir, "/:rw"),
			fmt.Sprint(cf3Path, ":", cf3Dir, "/:rw"),
		)

		if n.Cfg.License != "" && (n.isCPM("") || n.isStandaloneNode()) {
			// we mount a fixed path node.Labdir/license.key as the license referenced in topo file
			// will be copied to that path
			n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(
				filepath.Join(n.Cfg.LabDir, "license.key"), ":", licDir, "/license.txt:ro"))
		}

		return n.createSROSFiles()
	}
	return nil
}

// Post Deploy func for SR-SIM kind.
func (n *sros) PostDeploy(ctx context.Context, params *clabnodes.PostDeployParams) error {
	log.Info("Running postdeploy actions",
		"kind", n.Cfg.Kind,
		"node", n.Cfg.ShortName)

	// start waiting for container ready (PID based check)
	if err := n.Ready(ctx); err != nil {
		return err
	}

	// Disable TX checksum offload on the host NS veth for the mgmt interface.
	var peerIfIndex int
	err := n.ExecFunction(ctx, clabutils.VethPeerIndex("eth0", &peerIfIndex))
	if err != nil {
		log.Warn("Failed to get veth peer index for SR-SIM mgmt interface",
			"node", n.Cfg.ShortName,
			"error", err)
	} else {
		if err := clabutils.DisableTxOffloadByIndex(peerIfIndex); err != nil {
			log.Warn("Failed to disable TX checksum offload on SR-SIM mgmt host veth",
				"node", n.Cfg.ShortName,
				"error", err)
		}
	}

	errChan := make(chan error, 1)

	logs, err := n.Runtime.StreamLogs(ctx, n.GetContainerName())
	if err != nil {
		log.Debug("Failed to get container log stream", "node", n.Cfg.ShortName, "err", err)
	} else {
		// Start monitoring in a goroutine
		monitoringCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer logs.Close()

		go n.MonitorLogs(monitoringCtx, logs, errChan)
	}

	// Populate /etc/hosts for service discovery on mgmt interface
	if err = n.populateHosts(ctx, params.Nodes); err != nil {
		log.Warn("Unable to populate hosts list", "node", n.Cfg.ShortName, "err", err)
	}

	n.swVersion, err = n.RunningVersion(ctx)
	if err != nil {
		return err
	}
	if !n.isCPM(slotAName) {
		return nil
	}

	// Execute SaveConfig after boot. This code should only run on active CPM
	for time.Now().Before(time.Now().Add(readyTimeout)) {
		// Check if context is canceled
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context canceled: %w", err)
		}

		isHealthy, err := n.IsHealthy(ctx)
		if err != nil {
			log.Debug(
				fmt.Errorf(
					"health check failed, check 'docker logs -f %s': %w",
					n.Cfg.LongName,
					err,
				),
			)
		}

		if isHealthy {
			addr, err := n.MgmtIPAddr()
			if err != nil {
				return err
			}
			// TLS bootstrap in case of n.Cfg.Certificate.Issue flag
			if *n.Cfg.Certificate.Issue {
				log.Infof("TLS cert/key bootstrap for node %q", n.Cfg.LongName)

				err = n.tlsCertBootstrap(ctx, addr)
				if err != nil {
					return fmt.Errorf(
						"TLS cert/key bootstrap to node %q failed: %w",
						n.Cfg.LongName,
						err,
					)
				}
				log.Infof(
					"Completed bootstrap for gRPC-TLS profile on node %s",
					n.Cfg.ShortName,
				)
			}

			// Partial or NO Config Provided
			if !isFullConfigFile(n.Cfg.StartupConfig) {
				log.Infof("Saving node %q config as startup...", n.Cfg.LongName)
				err = n.saveConfigWithAddr(ctx, addr)
				if err != nil {
					return fmt.Errorf("save config to node %q, failed: %w", n.Cfg.LongName, err)
				}
			}
			return nil
		}

		// Wait before next check
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			log.With("kind", n.Cfg.Kind, "node", n.Cfg.ShortName).Debugf("got %q on errChan", err)
			log.Info("Skipping postdeploy actions", "kind", n.Cfg.Kind, "node", n.Cfg.ShortName)
			return nil
		case <-time.After(retryTimer):
			// continue to next iteration
		}
	}
	return nil
}

// Delete func for SR-SIM kind.
func (n *sros) Delete(ctx context.Context) error {
	// if not distributed, follow default node implementation
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.Runtime.DeleteContainer(ctx, n.GetContainerName())
	}

	// Delete all the component containers
	for _, componentNodes := range n.componentNodes {
		err := componentNodes.Delete(ctx)
		if err != nil {
			log.Warn(err)
		}
	}
	return nil
}

// DeleteNetnsSymlink deletes the symlink file created for the container netns.
func (n *sros) DeleteNetnsSymlink() error {
	// if it is the base node, then we need to delete the symlink for all the components.
	if n.isDistributedBaseNode() {
		for _, componentNode := range n.componentNodes {
			err := componentNode.DeleteNetnsSymlink()
			if err != nil {
				return err
			}
		}
		return nil
	}

	return n.DefaultNode.DeleteNetnsSymlink()
}

// sortComponents ensure components are in order of
// LCs first, then CPMs (cpm b comes first if present).
func (n *sros) sortComponents() {
	slices.SortFunc(n.Cfg.Components, func(a, b *clabtypes.Component) int {
		s1 := strings.ToUpper(strings.TrimSpace(a.Slot))
		s2 := strings.ToUpper(strings.TrimSpace(b.Slot))

		p1 := n.getSortOrder(s1)
		p2 := n.getSortOrder(s2)

		return p1 - p2
	})
}

func (n *sros) getSortOrder(slot string) int {
	r := rune(slot[0])
	if unicode.IsLetter(r) {
		// for letters, B before A, letters after numbers
		// B=66 -> 34
		// A=65 -> 35
		return 100 - int(r)
	} else {
		num, _ := strconv.Atoi(slot)
		return num // 1, 2, 3... smaller than CPM slots, so will come first
	}
}

func (n *sros) setupComponentNodes() error {
	if !n.isDistributedBaseNode() {
		return nil
	}

	n.sortComponents()

	// Registry, because it is not a package Var
	nr := clabnodes.NewNodeRegistry()
	Register(nr)

	// loop through the components, creating them
	for idx, c := range n.Cfg.Components {
		// instantiate a new nokia_srsim instance
		componentNode, err := nr.NewNodeOfKind("nokia_srsim")
		if err != nil {
			return err
		}

		// copy the original node's NodeConfig to the component
		componentConfig, err := deep.Copy(n.Cfg)
		if err != nil {
			return fmt.Errorf("failed to deep copy node config for component: %w", err)
		}

		// the first node will create the namespace, so NetworkMode remains unchanged.
		// all consecutive need to be attached to specifically that Namespace via NetworkMode
		if idx > 0 {
			componentConfig.NetworkMode = fmt.Sprintf("container:%s",
				n.componentNodes[0].GetShortName())
		}

		// adjust the config values from the original node
		componentConfig.ShortName = n.calcComponentName(componentConfig.ShortName, c.Slot)
		componentConfig.LongName = n.calcComponentName(componentConfig.LongName, c.Slot)
		componentConfig.NodeType = n.Cfg.NodeType
		componentConfig.Components = nil
		componentConfig.Fqdn = n.calcComponentFqdn(c.Slot)
		if idx != 0 {
			componentConfig.DNS = nil
			componentConfig.PortBindings = nil
			componentConfig.PortSet = nil
		}

		// add the component env to the componentConfig env
		for k, v := range c.Env {
			componentConfig.Env[k] = v
		}

		// set component environment variables
		n.setComponentEnvVars(componentConfig, c)
		componentConfig.Env[envNokiaSrosSlot] = c.Slot

		// adjust label based env vars
		componentConfig.Env["CLAB_LABEL_"+clabutils.ToEnvKey(clabconstants.NodeName)] = componentConfig.ShortName
		componentConfig.Env["CLAB_LABEL_"+clabutils.ToEnvKey(clabconstants.LongName)] = componentConfig.LongName

		if componentConfig.Labels == nil {
			componentConfig.Labels = map[string]string{}
		}

		// adjust labels
		componentConfig.Labels[clabconstants.NodeName] = componentConfig.ShortName
		componentConfig.Labels[clabconstants.LongName] = componentConfig.LongName
		componentConfig.Labels[clabconstants.RootNodeName] = n.Cfg.ShortName
		componentConfig.Labels[clabconstants.RootNodeLongName] = n.Cfg.LongName

		// init the component
		err = componentNode.Init(componentConfig)
		if err != nil {
			return err
		}
		// set the runtime by copying it from the general node
		componentNode.WithRuntime(n.GetRuntime())

		// store root components for cpms, for config gen
		if srosNode, ok := componentNode.(*sros); ok {
			srosNode.rootComponents = n.Cfg.Components
			// store base node name
			srosNode.baseShortName = n.Cfg.ShortName
			srosNode.baseLongName = n.Cfg.LongName
		}

		// store the node in the componentNodes
		n.componentNodes = append(n.componentNodes, componentNode)
	}
	return nil
}

// setComponentEnvVars sets environment variables for a component.
func (n *sros) setComponentEnvVars(componentConfig *clabtypes.NodeConfig, c *clabtypes.Component) {
	if c.Type != "" {
		componentConfig.Env[envNokiaSrosCard] = c.Type
	}

	if c.SFM != "" {
		componentConfig.Env[envNokiaSrosSFM] = c.SFM
	}

	for _, x := range c.XIOM {
		key := fmt.Sprintf("%s_X%d", envNokiaSrosXIOM, x.Slot)
		componentConfig.Env[key] = x.Type
		// add the nested MDA
		for _, m := range x.MDA {
			key := fmt.Sprintf("%s_X%d_%d", envNokiaSrosMDA, x.Slot, m.Slot)
			componentConfig.Env[key] = m.Type
		}
	}

	for _, m := range c.MDA {
		key := fmt.Sprintf("%s_%d", envNokiaSrosMDA, m.Slot)
		componentConfig.Env[key] = m.Type
	}
}

// deployFabric deploys the distributed SR-SIM when the `components` key is present.
func (n *sros) deployFabric(ctx context.Context, deployParams *clabnodes.DeployParams) error {
	// loop through the components, creating them
	for _, c := range n.componentNodes {
		c.PreDeploy(ctx, n.preDeployParams)
		// deploy the component
		err := c.Deploy(ctx, deployParams)
		if err != nil {
			return err
		}
	}

	cpmSlot, err := n.cpmSlot()
	if err != nil {
		return err
	}
	// store the CPM container name
	n.cpmContainerName = n.calcComponentName(n.Cfg.LongName, cpmSlot)
	n.renameDone = true

	// adjust also the mgmt IP addresses of the general node
	ips, err := n.distNodeMgmtIPs()
	if err != nil {
		return err
	}

	n.Cfg.MgmtIPv4Address = ips.IPv4
	n.Cfg.MgmtIPv6Address = ips.IPv6

	return nil
}

// isDistributedCard checks if the slot variable is set, hence it is an instance (slot) of a
// distributed setup.
// isDistributedCardNode returns true if this node is a card (linecard/IOM)
// in a distributed SR-SIM deployment. A distributed card has a slot assignment
// but no components of its own.
func (n *sros) isDistributedCardNode() bool {
	_, exists := n.Cfg.Env[envNokiaSrosSlot]
	return exists && len(n.Cfg.Components) == 0
}

// isDistributedBaseNode returns true if this is the base node of a distributed
// SR-SIM deployment. The base node orchestrates multiple component nodes.
func (n *sros) isDistributedBaseNode() bool {
	return len(n.Cfg.Components) > 1
}

// isStandaloneNode returns true if this is a standalone (non-distributed) SR-SIM node.
func (n *sros) isStandaloneNode() bool {
	return !n.isDistributedBaseNode() && !n.isDistributedCardNode()
}

// GetNSPath retrieves the Namespace Path.
func (n *sros) GetNSPath(ctx context.Context) (string, error) {
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.DefaultNode.GetNSPath(ctx)
	}
	// calculate cpm container name
	cpmSlot, err := n.cpmSlot()
	if err != nil {
		return "", err
	}
	cpmContainerName := n.calcComponentName(n.GetContainerName(), cpmSlot)
	nsp, err := n.Runtime.GetNSPath(ctx, cpmContainerName)
	if err != nil {
		log.Errorf("Unable to determine NetNS Path for node %s: %v", n.Cfg.ShortName, err)
		return "", err
	}
	return nsp, err
}

// calcComponentName appends the line card suffix to the given node name.
func (n *sros) calcComponentName(name, slot string) string {
	if n.renameDone {
		return name
	}
	return fmt.Sprintf("%s-%s", name, strings.ToLower(slot))
}

// calcComponentFqdn computes the FQDN for a given slot.
func (n *sros) calcComponentFqdn(slot string) string {
	if n.renameDone {
		return n.Cfg.Fqdn
	}
	fqdnDotIndex := strings.Index(n.Cfg.Fqdn, ".")
	result := fmt.Sprintf(
		"%s-%s%s",
		n.Cfg.Fqdn[:fqdnDotIndex],
		strings.ToLower(slot),
		n.Cfg.Fqdn[fqdnDotIndex:],
	)
	return result
}

// cpmNode returns a CPM Node (used in Distributed mode).
func (n *sros) cpmNode() (clabnodes.Node, error) {
	defaultSlot, err := n.cpmSlot()
	if err != nil {
		return nil, err
	}
	defaultSlot = strings.ToLower(defaultSlot)
	for _, cn := range n.componentNodes {
		if cn.GetShortName()[len(cn.GetShortName())-1:] == defaultSlot {
			return cn, nil
		}
	}
	return nil, fmt.Errorf("node %s: node for slot %s not found", n.GetShortName(), defaultSlot)
}

// cpmSlot returns the Slot of the preferred CPM Node (used when Distributed mode).
// It prefers slot A if present, otherwise returns slot B.
func (n *sros) cpmSlot() (string, error) {
	// Prefer slot A, fall back to slot B
	for _, comp := range n.Cfg.Components {
		if comp.Slot == slotAName {
			return slotAName, nil
		}
	}
	// Check for slot B as fallback
	for _, comp := range n.Cfg.Components {
		if comp.Slot == slotBName {
			return slotBName, nil
		}
	}
	return "", fmt.Errorf("node %s: unable to determine default slot", n.GetShortName())
}

// Deploy deploys the SR-SIM kind.
func (n *sros) Deploy(ctx context.Context, deployParams *clabnodes.DeployParams) error {
	// if it is a chassis with multiple cards (i.e. components)
	if n.isDistributedBaseNode() {
		err := n.deployFabric(ctx, deployParams)
		if err != nil {
			return err
		}

		// Update the nodes state
		n.SetState(clabnodesstate.Deployed)
		return nil
	}

	// if it is a regular node
	err := n.DefaultNode.Deploy(ctx, deployParams)
	if err != nil {
		return err
	}

	return nil
}

// Ready returns when the node boot sequence reached the stage when it is ready to accept config
// commands
// returns an error if not ready by the expiry of the timer readyTimeout.
func (n *sros) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	var err error
	readyCmds := []string{readyCmdCpm, readyCmdBoth, readyCmdIom}
	readyCmdsStrings := []string{"CPM", "BOTH", "IOM"}
	log.Debug("Waiting for SR OS node to boot...", "node", n.Cfg.ShortName)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"timed out waiting for SR OS node %s to boot: %v",
				n.Cfg.ShortName,
				err,
			)
		default:
			// check if cpm is running
			for k, cmd := range readyCmds {
				cmd, _ := clabexec.NewExecCmdFromString(cmd)
				execResult, err := n.RunExec(ctx, cmd)
				if err != nil || (execResult != nil && execResult.GetReturnCode() != 0) {
					logMsg := fmt.Sprintf(
						"status check %s failed on %s retrying",
						readyCmdsStrings[k],
						n.Cfg.ShortName,
					)
					if err != nil {
						logMsg += fmt.Sprintf(" error: %v", err)
					}
					if execResult != nil && execResult.GetReturnCode() != 0 {
						logMsg += fmt.Sprintf(
							", output:%s",
							strings.ReplaceAll(execResult.String(), "\n", "; "),
						)
					}
					log.Debug(logMsg)
				}
				if execResult != nil && execResult.GetReturnCode() == 0 {
					log.Debug("SR OS is ready to be configured", "node", n.Cfg.ShortName)
					return nil
				}
				time.Sleep(retryTimer)
			}
		}
	}
}

// checkKernelVersion emits a warning if the present kernel version is lower than the required one.
func (*sros) checkKernelVersion() error {
	// retrieve running kernel version
	kv, err := clabutils.GetKernelVersion()
	if err != nil {
		return err
	}

	// do the comparison
	if !kv.GreaterOrEqual(requiredKernelVersion) {
		log.Warnf(
			"Nokia SR OS requires a kernel version greater than %s. Detected kernel version: %s. Not all features might function properly!",
			requiredKernelVersion,
			kv,
		)
	}
	return nil
}

// checkComponentSlotsConfig check the sros component slot config for validity.
func (n *sros) checkComponentSlotsConfig() error {
	// check Slots are unique
	componentNames := map[string]struct{}{}
	for _, component := range n.Cfg.Components {
		// convert slot to upper
		slot := strings.ToUpper(component.Slot)
		// check if slot exists
		_, exists := componentNames[slot]
		if exists {
			return fmt.Errorf(
				"node %s slot %s duplicate definition",
				n.GetShortName(),
				component.Slot,
			)
		}
		// add to component names map
		componentNames[component.Slot] = struct{}{}

	}
	return nil
}

func (n *sros) CheckDeploymentConditions(ctx context.Context) error {
	// perform the sros specific kernel version check
	err := n.checkKernelVersion()
	if err != nil {
		return err
	}

	// check the sros component slot config for validaity
	err = n.checkComponentSlotsConfig()
	if err != nil {
		return err
	}

	return n.DefaultNode.CheckDeploymentConditions(ctx)
}

// Func that creates the Dirs used for the kind SR-SIM and sets/merges the default Env vars.
func (n *sros) createSROSFiles() error {
	log.Debug("Creating directory structure for SR OS container", "node", n.Cfg.ShortName)

	var err error

	if n.Cfg.License != "" && (n.isCPM("") || n.isStandaloneNode()) {
		// copy license file to node specific directory in lab
		licPath := filepath.Join(n.Cfg.LabDir, "license.key")
		if err := clabutils.CopyFile(context.Background(), n.Cfg.License, licPath,
			clabconstants.PermissionsFileDefault); err != nil {
			return fmt.Errorf(
				"license copying src %s -> dst %s failed: %v",
				n.Cfg.License,
				licPath,
				err,
			)
		}
		log.Debug("SR OS license copied", "src", n.Cfg.License, "dst", licPath)
	}
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot]),
		clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], "config"),
		clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf1),
		clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf2),
		clabconstants.PermissionsOpen)
	clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf3),
		clabconstants.PermissionsOpen)
	if n.isCPM(slotAName) || n.isStandaloneNode() {
		err = n.createSROSCertificates()
	}
	if err != nil {
		return err
	}
	// Skip config if node is not CPM
	if n.isCPM("") || n.isStandaloneNode() {
		err = n.createSROSConfigFiles()
		if err != nil {
			return err
		}
	}
	return nil
}

// Func that Places the Certificates in the right place and format.
func (n *sros) createSROSCertificates() error {
	if *n.Cfg.Certificate.Issue {
		clabutils.CreateDirectory(path.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf3),
			clabconstants.PermissionsOpen)
		keyPath := filepath.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf3, tlsKeyFile)
		if err := clabutils.CreateFile(keyPath, n.Config().TLSKey); err != nil {
			return err
		}

		certPath := filepath.Join(n.Cfg.LabDir, n.Cfg.Env[envNokiaSrosSlot], configCf3, tlsCertFile)
		err := clabutils.CreateFile(certPath, n.Config().TLSCert)
		if err != nil {
			return err
		}
	}
	return nil
}

// createSROSConfigFiles handles config generation for the SR-SIM kind.
// Flow: version detection → buildStartupConfig (default + partial) → GenerateConfig(dst, config).
func (n *sros) createSROSConfigFiles() error {
	// Get version from image before generating config
	if n.swVersion == nil {
		ctx := context.Background()
		version, err := n.srosVersionFromImage(ctx)
		if err != nil {
			log.Warn("Failed to get SR OS version from image",
				"node", n.Cfg.ShortName, "error", err)
		} else {
			n.swVersion = version
			log.Info("Retrieved SR OS version from image",
				"node", n.Cfg.ShortName,
				"version", fmt.Sprintf("%s.%s.%s", version.Major, version.Minor, version.Build))
		}
	}

	// Path pointing to the target config file under configCf3 dir
	cf3CfgFile := filepath.Join(
		n.Cfg.LabDir,
		n.Cfg.Env[envNokiaSrosSlot],
		configCf3,
		startupCfgName,
	)
	isPartial := clabutils.IsPartialConfigFile(n.Cfg.StartupConfig)

	// generate config and use that to boot node
	log.Debug("Reading startup-config", "node", n.Cfg.ShortName, "startup-config",
		n.Cfg.StartupConfig, "isPartial", isPartial)

	startupConfig, err := n.buildStartupConfig(isPartial)
	if err != nil {
		return err
	}

	if startupConfig == "" {
		log.Debug(
			"startup config is empty, skipping startup config file generation",
			"node",
			n.Cfg.ShortName,
		)
		return nil
	}

	return n.GenerateConfig(cf3CfgFile, startupConfig)
}

// buildStartupConfig returns the full startup config string: either from user file (full config)
// or from default + partial config generation. It does not return a template; the name is
// historical.
func (n *sros) buildStartupConfig(isPartial bool) (string, error) {
	// User provides full startup config
	if n.Cfg.StartupConfig != "" && !isPartial {
		c, err := os.ReadFile(n.Cfg.StartupConfig)
		if err != nil {
			return "", err
		}

		cBuf, err := clabutils.SubstituteEnvsAndTemplate(bytes.NewReader(c), n.Cfg)
		if err != nil {
			return "", err
		}
		return cBuf.String(), nil
	}

	// Generate default config and optionally add partial config
	if err := n.addDefaultConfig(); err != nil {
		return "", err
	}
	if err := n.addPartialConfig(); err != nil {
		return "", err
	}

	return string(n.startupCliCfg), nil
}

// SlotIsInteger checks if the slot string represents a valid integer.
func SlotIsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// Check if a container is a CPM.
// isCPM checks if a container is a CPM (Control Processing Module).
// Returns false if the slot is a linecard (integer slot).
// If a specific CPM name is provided, returns false if this is a different CPM.
func (n *sros) isCPM(cpm string) bool {
	slot, exists := n.Cfg.Env[envNokiaSrosSlot]
	if !exists {
		// No slot specified - this is a CPM
		return true
	}

	// If slot is an integer, it's a linecard, not a CPM
	if SlotIsInteger(slot) {
		return false
	}

	// Slot is non-integer (CPM slot). If a specific CPM was requested,
	// check if this is that CPM
	if cpm != "" && !strings.EqualFold(slot, cpm) {
		return false
	}

	return true
}

// prepareConfigTemplateData prepares all data needed for template selection and execution.
// Service configs are filled from a single table-driven result: variant → getFullSnippetSet(v).
func (n *sros) prepareConfigTemplateData() (*srosTemplateData, error) {
	b, err := n.banner()
	if err != nil {
		return nil, err
	}

	componentConfig := ""
	if !isFullConfigFile(n.Cfg.StartupConfig) {
		componentConfig = n.generateComponentConfig()
	} else {
		log.Debugf("SR-SIM node %q has non-partial startup-config defined, skipping component config gen", n.Cfg.LongName)
	}

	v := n.resolveConfigVariant()
	snippets := getFullSnippetSet(v)
	configMode := string(v.Mode)
	if v.ForceClassic {
		log.Warn(
			"SAR-Hm nodes only support classic configuration mode. Overriding configuration mode to 'classic'",
			"node", n.Cfg.LongName,
			"node-type", strings.ToLower(n.Cfg.NodeType),
		)
		configMode = string(ConfigModeClassic)
		n.Cfg.Env[envSrosConfigMode] = string(ConfigModeClassic)
	}

	tplData := &srosTemplateData{
		// Selection criteria
		NodeType:          strings.ToLower(n.Cfg.NodeType),
		ConfigurationMode: configMode,
		SwVersion:         n.swVersion,
		IsSecureGrpc:      *n.Cfg.Certificate.Issue,

		// Node data
		Name:            n.Cfg.ShortName,
		TLSKey:          n.Cfg.TLSKey,
		TLSCert:         n.Cfg.TLSCert,
		TLSAnchor:       n.Cfg.TLSAnchor,
		Banner:          b,
		IFaces:          map[string]tplIFace{},
		MgmtMTU:         0,
		MgmtIPMTU:       n.Runtime.Mgmt().MTU,
		ComponentConfig: componentConfig,

		// Service configs from variant (single source of truth)
		SystemConfig:  snippets.SystemConfig,
		GRPCConfig:    snippets.GRPCConfig,
		SNMPConfig:    snippets.SNMPConfig,
		NetconfConfig: snippets.NetconfConfig,
		LoggingConfig: snippets.LoggingConfig,
		SSHConfig:     snippets.SSHConfig,
	}

	if n.Config().DNS != nil {
		tplData.DNSServers = append(tplData.DNSServers, n.Config().DNS.Servers...)
	}

	n.prepareSSHPubKeys(tplData)
	return tplData, nil
}

// isIXRNode returns true if this is an IXR node type (case-insensitive).
func (n *sros) isIXRNode() bool {
	return ixrRegexp.MatchString(n.Cfg.NodeType)
}

// isSARNode returns true if this is a SAR node type (case-insensitive).
func (n *sros) isSARNode() bool {
	return sarRegexp.MatchString(n.Cfg.NodeType)
}

// isSARHmNode returns true if this is a SAR-Hm or SAR-Hmc node type (case-insensitive).
func (n *sros) isSARHmNode() bool {
	return sarHmRegexp.MatchString(n.Cfg.NodeType)
}

// getTemplateForVariant returns the template string and name for the given config variant.
// swVersion is for future SR OS 26+ template selection (e.g. cfgTplSROS26 when Major >= "26").
func getTemplateForVariant(v ConfigVariant, swVersion *SrosVersion) (tmpl string, tplName string) {
	// Model-driven (or mixed treated as classic below when family is classic)
	if v.Mode == ConfigModeModelDriven {
		// Placeholder for version-specific template: if swVersion != nil && swVersion.Major >= "26" { return cfgTplSROS26, "clab-sros-config-sros26" }
		return cfgTplSROS25, "clab-sros-config-sros25"
	}
	// Classic (or mixed)
	tmpl = cfgTplClassic
	tplName = "clab-sros-config-classic"
	switch v.Family {
	case ConfigFamilyIXR:
		return cfgTplClassicIxr, "clab-sros-config-classic-ixr"
	case ConfigFamilySAR:
		return cfgTplClassicSar, "clab-sros-config-classic-sar"
	default:
		return tmpl, tplName
	}
}

// selectConfigTemplate chooses the config template from the config variant (mode + family).
func (n *sros) selectConfigTemplate(tplData *srosTemplateData) (*template.Template, error) {
	v := n.resolveConfigVariant()
	tmpl, tplName := getTemplateForVariant(v, tplData.SwVersion)
	return template.New(tplName).
		Funcs(clabutils.CreateFuncs()).
		Parse(tmpl)
}

// addDefaultConfig adds sros default configuration such as tls certs, gnmi/json-rpc, login-banner,
// ssh keys.
func (n *sros) addDefaultConfig() error {
	// Prepare all template data
	tplData, err := n.prepareConfigTemplateData()
	if err != nil {
		return err
	}

	// Select appropriate template
	srosCfgTpl, err := n.selectConfigTemplate(tplData)
	if err != nil {
		return fmt.Errorf("failed to select config template: %w", err)
	}
	log.Debug("Prepare SR OS config template", "template", srosCfgTpl.Name(),
		"node", n.Cfg.LongName,
		"configuration-mode", tplData.ConfigurationMode,
		"node-type", tplData.NodeType,
		"secure-grpc", tplData.IsSecureGrpc,
		"sw-version", tplData.SwVersion)

	// Execute template
	buf := new(bytes.Buffer)
	err = srosCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}

	if buf.Len() == 0 {
		log.Warn(
			"Buffer empty, template parsing error",
			"node", n.Cfg.ShortName,
			"template", srosCfgTpl.Name(),
		)
	} else {
		log.Debug("Additional default config parsed",
			"node", n.Cfg.ShortName,
			"template", srosCfgTpl.Name())
		n.startupCliCfg = append(n.startupCliCfg, buf.String()...)
	}

	return nil
}

// applyPartialConfig applies partial configuration to the SR OS.
func (n *sros) addPartialConfig() error {
	if n.Cfg.StartupConfig != "" {
		// b holds the configuration to be applied to the node
		b := &bytes.Buffer{}
		// apply partial configs if partial config is used
		if clabutils.IsPartialConfigFile(n.Cfg.StartupConfig) && n.isCPM("") {
			log.Info("Adding configuration",
				"node", n.Cfg.LongName,
				"type", "partial",
				"source", n.Cfg.StartupConfig)

			r, err := os.Open(n.Cfg.StartupConfig)
			if err != nil {
				return err
			}

			defer r.Close() // skipcq: GO-S2307

			_, err = io.Copy(b, r)
			if err != nil {
				return err
			}

			configContent, err := clabutils.SubstituteEnvsAndTemplate(b, n.Cfg)
			if err != nil {
				return err
			}
			if configContent.Len() == 0 {
				log.Warn(
					"Buffer empty, PARTIAL config template parsing error",
					"node",
					n.Cfg.ShortName,
				)
			} else {
				log.Debug("Additional PARTIAL config parsed", "node",
					n.Cfg.ShortName, "partial-config", configContent.String())
				n.startupCliCfg = append(n.startupCliCfg, configContent.String()...)
			}
		} else {
			log.Warn("Passed startup-config option, but it will not have any effect", "node", n.Cfg.ShortName)
		}
	}
	return nil
}

func (n *sros) GetContainers(ctx context.Context) ([]clabruntime.GenericContainer, error) {
	// if not a distributed setup call regular GetContainers
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.DefaultNode.GetContainers(ctx)
	}

	cpmSlot, err := n.cpmSlot()
	if err != nil {
		return nil, err
	}
	containerName := n.calcComponentName(n.GetContainerName(), cpmSlot)

	cnts, err := n.Runtime.ListContainers(ctx, []*clabtypes.GenericFilter{
		{
			FilterType: "name",
			Match:      containerName,
		},
	})
	if err != nil {
		return nil, err
	}
	// check that we retrieved some container information
	// otherwise throw ErrContainersNotFound error
	if len(cnts) == 0 {
		return nil, fmt.Errorf("node: %s. %w", n.GetContainerName(),
			clabnodes.ErrContainersNotFound)
	}

	// Forge the IP address to be the actual IP of mgmt
	// because the CPM A might not own the netns & mgmt IP
	if len(n.Cfg.Components) > 0 {
		ips, err := n.distNodeMgmtIPs()
		if err == nil {
			if ips.IPv4 != "" {
				cnts[0].NetworkSettings.IPv4addr = ips.IPv4
				cnts[0].NetworkSettings.IPv4pLen = ips.IPv4pLen
			}
			if ips.IPv6 != "" {
				cnts[0].NetworkSettings.IPv6addr = ips.IPv6
				cnts[0].NetworkSettings.IPv6pLen = ips.IPv6pLen
			}
		}
	}

	return cnts, err
}

// populateHosts adds container hostnames for other nodes of a lab to SR Linux /etc/hosts file
// to mitigate the fact that srlinux uses non default netns for management and thus
// can't leverage docker DNS service.
func (n *sros) populateHosts(ctx context.Context, nodes map[string]clabnodes.Node) error {
	containerName := n.Cfg.LongName
	if n.isDistributedBaseNode() && n.cpmContainerName != "" {
		containerName = n.cpmContainerName
	}

	hosts, err := n.Runtime.GetHostsPath(ctx, containerName)
	if err != nil {
		log.Warn("Unable to locate SR OS node /etc/hosts file", "node", n.Cfg.ShortName, "err", err)
		return err
	}
	var entriesv4, entriesv6 bytes.Buffer
	const (
		v4Prefix = "###### CLAB-v4-START ######"
		v4Suffix = "###### CLAB-v4-END ######"
		v6Prefix = "###### CLAB-v6-START ######"
		v6Suffix = "###### CLAB-v6-END ######"
	)
	fmt.Fprintf(&entriesv4, "\n%s\n", v4Prefix)
	fmt.Fprintf(&entriesv6, "\n%s\n", v6Prefix)
	for node, params := range nodes {
		if v4 := params.Config().MgmtIPv4Address; v4 != "" {
			fmt.Fprintf(&entriesv4, "%s\t%s\n", v4, node)
		}
		if v6 := params.Config().MgmtIPv6Address; v6 != "" {
			fmt.Fprintf(&entriesv6, "%s\t%s\n", v6, node)
		}
	}
	fmt.Fprintf(&entriesv4, "%s\n", v4Suffix)
	fmt.Fprintf(&entriesv6, "%s\n", v6Suffix)

	file, err := os.OpenFile(hosts, os.O_APPEND|os.O_WRONLY, 0o666) // skipcq: GSC-G302
	if err != nil {
		log.Warn("Unable to open SR OS node /etc/hosts file", "node", n.Cfg.ShortName, "err", err)
		return err
	}

	_, err = file.Write(entriesv4.Bytes())
	if err != nil {
		return err
	}
	_, err = file.Write(entriesv6.Bytes())
	if err != nil {
		return err
	}

	return file.Close()
}

func (n *sros) GetMappedInterfaceName(ifName string) (string, error) {
	captureGroups, err := clabutils.GetRegexpCaptureGroups(n.InterfaceRegexp, ifName)
	if err != nil {
		return "", err
	}

	indexGroups := []string{"card", "xiom", "mda", "connector", "port"}
	parsedIndices := make(map[string]int)
	foundIndices := make(map[string]bool)

	for _, indexKey := range indexGroups {
		if index, found := captureGroups[indexKey]; found && index != "" {
			foundIndices[indexKey] = true
			parsedIndices[indexKey], err = strconv.Atoi(index)
			if err != nil {
				return "", fmt.Errorf(
					"%q parsed %s index %q could not be cast to an integer",
					ifName,
					indexKey,
					index,
				)
			}
			if parsedIndices[indexKey] < 1 {
				return "", fmt.Errorf(
					"%q parsed %q index %q does not match requirement >= 1",
					ifName,
					indexKey,
					index,
				)
			}
		} else {
			foundIndices[indexKey] = false
		}
	}

	// Card, MDA and port are present
	if foundIndices["card"] && foundIndices["mda"] && foundIndices["port"] {
		switch {
		case foundIndices["xiom"] && foundIndices["connector"]:
			// XIOM and connector are present, format will be: e1-x2-3-c4-5 -> card 1, xiom 2, mda
			// 3, connector 4, port 5
			return fmt.Sprintf(
				"e%d-x%d-%d-c%d-%d",
				parsedIndices["card"],
				parsedIndices["xiom"],
				parsedIndices["mda"],
				parsedIndices["connector"],
				parsedIndices["port"],
			), nil

		case foundIndices["xiom"]:
			// Only XIOM present, format will be: e1-x2-3-4 -> card 1, xiom 2, mda 3, port 4
			return fmt.Sprintf("e%d-x%d-%d-%d", parsedIndices["card"],
				parsedIndices["xiom"], parsedIndices["mda"], parsedIndices["port"]), nil
		case foundIndices["connector"]:
			// Only connector present, format will be: e1-2-c3-4 -> card 1, mda 2, connector 3, port
			// 4
			return fmt.Sprintf("e%d-%d-c%d-%d", parsedIndices["card"],
				parsedIndices["mda"], parsedIndices["connector"], parsedIndices["port"]), nil
		default:
			// No XIOM or connector present, format will be: e1-2-3 -> card 1, mda 2, port 3
			return fmt.Sprintf("e%d-%d-%d", parsedIndices["card"],
				parsedIndices["mda"], parsedIndices["port"]), nil
		}
	} else {
		return "", fmt.Errorf("%q missing card, mda or port index", ifName)
	}
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *sros) CheckInterfaceName() error {
	nm := n.Cfg.NetworkMode

	err := n.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, e := range n.Endpoints {
		if !MappedInterfaceRegexp.MatchString(e.GetIfaceName()) {
			return fmt.Errorf(
				"nokia SR OS interface name %q doesn't match the required pattern: %s",
				e.GetIfaceName(),
				n.InterfaceHelp,
			)
		}

		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf(
				"eth0 interface name is not allowed for %s node when network mode is not set to none",
				n.Cfg.ShortName,
			)
		}
	}

	return nil
}

func (n *sros) SaveConfig(ctx context.Context) (*clabnodes.SaveConfigResult, error) {
	fqdn := ""
	switch {
	case n.isStandaloneNode():
		// check if it is a cpm node. return without error if not
		if !n.isCPM("") {
			return nil, nil
		}
		// if it is a standalone node use the fqdn
		fqdn = n.Cfg.Fqdn
	case n.isDistributedBaseNode():
		// if it is the
		cmpNode, err := n.cpmNode()
		if err != nil {
			return nil, err
		}
		// delegate to cpm node
		return cmpNode.SaveConfig(ctx)
	case n.isDistributedCardNode():
		// check if it is a cpm node. return without error if not
		if !n.isCPM("") {
			return nil, nil
		}
		fqdn = n.Cfg.LongName
	}

	if err := n.saveConfigWithAddr(ctx, fqdn); err != nil {
		return nil, err
	}

	cfgPath := filepath.Join(
		n.Cfg.LabDir,
		n.Cfg.Env[envNokiaSrosSlot],
		configCf3,
		startupCfgName,
	)

	return &clabnodes.SaveConfigResult{
		ConfigPath: cfgPath,
	}, nil
}

// saveConfigWithAddr will use the addr string to try to save the config of the node.
func (n *sros) saveConfigWithAddr(ctx context.Context, addr string) error {
	if n.isConfigClassic() {
		cmd := []string{"/admin save", "/bof persist on", "/bof save"}
		return n.srosSendCommandsSSH(ctx, scrapliPlatformNameClassic, cmd)
	}
	err := clabnetconf.SaveRunningConfig(fmt.Sprintf("[%s]", addr),
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		scrapliPlatformName,
	)
	if err != nil {
		return err
	}

	log.Info(
		"Saved running configuration",
		"node",
		n.Cfg.ShortName,
		"addr",
		addr,
		"config-mode",
		n.Cfg.Env[envSrosConfigMode],
	)

	return nil
}

// BuildPKIImportXML.
func buildPKIImportXML(inputURL, outputFile, importType string) string {
	action := etree.NewElement("action")
	action.CreateAttr("xmlns", "urn:ietf:params:xml:ns:yang:1")

	admin := action.CreateElement("admin")
	admin.CreateAttr("xmlns", "urn:nokia.com:sros:ns:yang:sr:oper-admin")

	importElem := admin.CreateElement("system").
		CreateElement("security").
		CreateElement("pki").
		CreateElement("import")

	// Add import parameters
	importElem.CreateElement("input-url").SetText(inputURL)
	importElem.CreateElement("output-file").SetText(outputFile)
	importElem.CreateElement("type").SetText(importType)
	importElem.CreateElement("format").SetText("pem")
	var buf bytes.Buffer
	action.WriteTo(&buf, &etree.WriteSettings{
		CanonicalText:    false,
		CanonicalAttrVal: false,
	})
	return buf.String()
}

// BuildTLSProfileXML builds the TLS profile configuration XML.
func buildTLSProfileXML() string {
	config := etree.NewElement("config")
	configure := config.CreateElement("configure")
	configure.CreateAttr("xmlns", "urn:nokia.com:sros:ns:yang:sr:conf")
	configure.CreateAttr("xmlns:nc", "urn:ietf:params:xml:ns:netconf:base:1.0")

	certProfile := configure.CreateElement("system").
		CreateElement("security").
		CreateElement("tls").
		CreateElement("cert-profile")

	// Set operation attribute
	certProfile.CreateAttr("nc:operation", "merge")

	// Add profile configuration
	certProfile.CreateElement("cert-profile-name").SetText(tlsCertProfileName)
	certProfile.CreateElement("admin-state").SetText("enable")

	var buf bytes.Buffer
	config.WriteTo(&buf, &etree.WriteSettings{
		CanonicalText:    false,
		CanonicalAttrVal: false,
	})
	return buf.String()
}

// TLS bootstrap via NETCONF to enable secure gRPC.
func (n *sros) tlsCertBootstrap(ctx context.Context, addr string) error {
	// Always import PKI key and cert:
	// 	 import "cf3:\node.key" in PEM format as "cf3:\system-pki\node.key" (encrypted DER)
	//   import "cf3:\node.crt" in PEM format as "cf3:\system-pki\node.crt" (encrypted DER)
	operations := []clabnetconf.Operation{
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.RPC(opoptions.WithFilter(buildPKIImportXML(
				fmt.Sprintf("cf3:/%s", tlsKeyFile), tlsKeyFile, "key")))
		},
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.RPC(opoptions.WithFilter(buildPKIImportXML(
				fmt.Sprintf("cf3:/%s", tlsCertFile), tlsCertFile, "certificate")))
		},
	}

	// Activate cert-profile in MD is via NETCONF, in Classic mode is via SSH
	//  enable enables cert-profile "clab-grpc-certs" administratively
	cmd := []string{}
	if n.isConfigClassic() {
		cmd = append(
			cmd,
			fmt.Sprintf(
				"/configure system security tls cert-profile %s no shutdown",
				tlsCertProfileName,
			),
		)
	} else {
		operations = append(operations,
			func(d *netconf.Driver) (*response.NetconfResponse, error) {
				return d.EditConfig("candidate", buildTLSProfileXML())
			},
			func(d *netconf.Driver) (*response.NetconfResponse, error) {
				return d.Commit()
			},
		)
	}

	err := clabnetconf.MultiExec(
		fmt.Sprintf("[%s]", addr),
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		operations,
	)
	if len(cmd) > 0 && n.isConfigClassic() {
		err := n.srosSendCommandsSSH(ctx, scrapliPlatformNameClassic, cmd)
		if err != nil {
			return err
		}
	}
	return err
}

// isConfigClassic returns true if the env var for configuration contains "mixed" or "classic"
// strings.
func (n *sros) isConfigClassic() bool {
	cfgMode := strings.ToLower(n.Cfg.Env[envSrosConfigMode])
	return cfgMode == "classic" || cfgMode == "mixed"
}

// isFullConfigFile returns true if the config file doesn't contain .partial substring
// and the config file is NOT nil (ie. startup config IS defined).
// it is intended that the 'c' arg is n.Cfg.StartupConfig.
func isFullConfigFile(c string) bool {
	return c != "" && !clabutils.IsPartialConfigFile(c)
}

func (n *sros) IsHealthy(_ context.Context) (bool, error) {
	if !n.isCPM("") {
		return true, fmt.Errorf("node %q is not a CPM, healthcheck has no effect", n.Cfg.LongName)
	}
	// non-partial user startup config might not have any netconf config
	// so we shouldn't check for this.
	if isFullConfigFile(n.Cfg.StartupConfig) {
		log.Debug(
			"node has full startup config, skipping NETCONF check",
			"kind",
			n.Cfg.Kind,
			"node",
			n.Cfg.ShortName,
		)
		return true, nil
	}
	addr, err := n.MgmtIPAddr()
	if err != nil {
		return false, err
	}
	log.Debug(
		"Checking netconf connection",
		"node",
		n.Cfg.LongName,
		"addr",
		fmt.Sprintf("%q:830", addr),
	)
	return CheckPortWithRetry(addr, 830, readyTimeout, 5, retryTimer)
}

// CheckPortWithRetry checks if a port is open with retry logic.
func CheckPortWithRetry(
	host string,
	port int,
	timeout time.Duration,
	maxRetries int,
	retryDelay time.Duration,
) (bool, error) {
	var lastErr error

	for i := range maxRetries {
		if i > 0 {
			time.Sleep(retryDelay)
		}

		address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			conn.Close()
			return true, nil
		}
		lastErr = err
	}

	return false, lastErr
}

// MgmtIP represents the management IPv4/v6 addresses of a node.
type MgmtIP struct {
	IPv4        string
	IPv4pLen    int
	IPv6        string
	IPv6pLen    int
	ContainerID string
}

// distNodeMgmtIPs returns both ipv4 and ipv6 management IP address of a
// distributed node defined via components.
// It returns an error if neither is set in the node config.
func (n *sros) distNodeMgmtIPs() (MgmtIP, error) {
	ips := MgmtIP{}

	var components []*clabtypes.Component

	if n.isDistributedBaseNode() {
		components = n.Cfg.Components
	} else {
		components = n.rootComponents
	}

	// for distributed nodes, get the IP from the
	// 0th component container
	if len(components) > 0 {
		c := components[0]
		slot := strings.ToLower(c.Slot)

		componentName := n.Cfg.LongName + "-" + slot

		containers, err := n.Runtime.ListContainers(
			context.Background(),
			[]*clabtypes.GenericFilter{
				{
					FilterType: "name",
					Match:      componentName,
				},
			},
		)
		if err != nil {
			return MgmtIP{}, fmt.Errorf("unable to get container %q: %v", componentName, err)
		}

		for _, container := range containers {
			if container.NetworkSettings.IPv4addr != "" {
				ips.IPv4 = container.NetworkSettings.IPv4addr
				ips.IPv4pLen = container.NetworkSettings.IPv4pLen
				ips.ContainerID = container.ID
			}
			if container.NetworkSettings.IPv6addr != "" {
				ips.IPv6 = container.NetworkSettings.IPv6addr
				ips.IPv6pLen = container.NetworkSettings.IPv6pLen
				ips.ContainerID = container.ID
			}

		}
	}

	return ips, nil
}

// custom override for hosts file entry to write basename when using component nodes.
func (n *sros) GetHostsEntries(ctx context.Context) (clabtypes.HostEntries, error) {
	result, err := n.DefaultNode.GetHostsEntries(ctx)
	if err != nil {
		return nil, err
	}

	if n.isDistributedBaseNode() {
		ips, err := n.distNodeMgmtIPs()
		if err != nil {
			return clabtypes.HostEntries{}, err
		}

		if ips.IPv4 != "" { // v4
			result = append(result, clabtypes.NewHostEntry(
				ips.IPv4,
				n.Cfg.LongName,
				clabtypes.IpVersionV4,
			).SetDescription(fmt.Sprintf("Kind: %s", n.Cfg.Kind)).SetContainerID(ips.ContainerID))
		}

		if ips.IPv6 != "" {
			result = append(result, clabtypes.NewHostEntry(
				ips.IPv6,
				n.Cfg.LongName,
				clabtypes.IpVersionV6,
			).SetDescription(fmt.Sprintf("Kind: %s", n.Cfg.Kind)).SetContainerID(ips.ContainerID))
		}
	}

	return result, nil
}

// MgmtIPAddr returns ipv4 or ipv6 management IP address of the node.
// It returns an error if neither is set in the node config.
func (n *sros) MgmtIPAddr() (string, error) {
	switch {
	case n.Cfg.MgmtIPv6Address != "":
		return n.Cfg.MgmtIPv6Address, nil
	case n.Cfg.MgmtIPv4Address != "":
		return n.Cfg.MgmtIPv4Address, nil
	}

	if !n.isStandaloneNode() {
		ips, err := n.distNodeMgmtIPs()
		if err != nil {
			return "", err
		}

		switch {
		case ips.IPv6 != "":
			return ips.IPv6, nil
		case ips.IPv4 != "":
			return ips.IPv4, nil
		}
	}

	return n.Cfg.LongName, fmt.Errorf(
		"no management IP address (IPv4 or IPv6) configured for node %q",
		n.Cfg.LongName,
	)
}

// generateComponentConfig generates SR OS configuration for explicitly defined
// components (cards, SFMs, XIOMs, MDAs) using a struct + loop; power config is appended.
func (n *sros) generateComponentConfig() string {
	if _, exists := n.Cfg.Env[envDisableComponentConfigGen]; exists {
		return ""
	}
	if n.isConfigClassic() {
		return ""
	}
	if n.isStandaloneNode() {
		return ""
	}
	components := n.rootComponents
	if len(components) == 0 {
		return ""
	}

	for _, c := range components {
		slot := strings.ToUpper(strings.TrimSpace(c.Slot))
		if slot == slotAName || slot == slotBName {
			continue
		}
		if c.Type == "" {
			log.Warn("SR-SIM node has no type set for component in slot, skipping component SR OS config generation.",
				"node", n.Cfg.ShortName, "slot", slot)
		}
	}

	lines := buildComponentCfgLines(components)
	var config strings.Builder
	for _, l := range lines {
		config.WriteString(l.String())
	}
	config.WriteString(n.generatePowerConfig())
	return config.String()
}

// override to fetch CPM to avoid renaming the base node to CPM A.
func (n *sros) GetContainerName() string {
	if n.isDistributedBaseNode() && n.cpmContainerName != "" {
		return n.cpmContainerName
	}
	return n.DefaultNode.GetContainerName()
}

// MonitorLogs monitors log output from the provided reader during the PostDeploy phase of SRSIM.
// It scans each line for SR OS error messages (minor or critical) and logs them as warnings
// The method checks for context cancellation on each line and returns immediately if the
// context is cancelled.
// Parameters:
//   - ctx: context for cancellation; if cancelled, the method returns.
//   - reader: io.ReadCloser providing log lines to scan.
//   - exitChan: channel which will signal an error for postdeploy to skip.
//
// The method returns when the context is cancelled, when the reader is exhausted or if
// it detects that SR OS rejected the config as per the const srosRejectedCfgMsg.
func (n *sros) MonitorLogs(ctx context.Context, reader io.ReadCloser, exitChan chan<- error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		line = clabutils.StripNonPrintChars(line)

		if strings.Contains(line, srosMinorError) ||
			strings.Contains(line, srosCriticalError) {
			log.Warn(
				"Got SR OS log message",
				"node",
				n.Cfg.ShortName,
				"message",
				line+"\n",
			)
		}

		if strings.Contains(line, srosRejectedCfgMsg) {
			exitChan <- errors.New("configuration rejected by SR OS")
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// verifyNokiaSrosImage ensures the image used with kind nokia_srsim has the correct labels
// It inspects the image label
// org.opencontainers.image.title and returns an error if it is not "srsim".
func (n *sros) verifyNokiaSrsimImage(ctx context.Context) error {
	if n.GetRuntime() == nil {
		return nil
	}
	insp, err := n.GetRuntime().InspectImage(ctx, n.Cfg.Image)
	if err != nil {
		// Skip check when runtime does not support image inspection (e.g. Podman).
		if strings.Contains(err.Error(), "not implemented") {
			log.Debug("Skipping nokia_srsim image kind check: runtime does not support image inspection")
			return nil
		}
		return err
	}
	if insp != nil && insp.Config.Labels != nil {
		if _, hasVrnetlab := insp.Config.Labels[vrnetlabVersionLabel]; hasVrnetlab {
			return fmt.Errorf(
				"node %q: kind is nokia_srsim but the image is a vrnetlab image; use kind: nokia_sros with this image, or use the SR-SIM container image for kind: nokia_srsim",
				n.Cfg.ShortName,
			)
		}
		if title, ok := insp.Config.Labels[srosImageTitleLabel]; ok && title == srosImageTitle {
			return nil
		}
	}
	log.Warnf("node %q: kind is nokia_srsim but the provided image does not have the correct labels; please use a valid SR-SIM container image or run a more recent version of the SR-SIM container image to suppress this warning", n.Cfg.ShortName)
	return nil
}

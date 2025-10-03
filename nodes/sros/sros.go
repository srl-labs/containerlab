// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

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

	defaultSrosPowerType       = "dc"
	defaultSrosPowerModuleType = "ps-a-dc-6000"
)

var (

	//go:embed sros_default_config.go.tpl
	srosConfigCmdsTpl string
	kindNames         = []string{"nokia_srsim"}
	srosSysctl        = map[string]string{
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
	}

	readyCmdCpm  = `/usr/bin/pgrep ^cpm$`
	readyCmdBoth = `/usr/bin/pgrep ^both$`
	readyCmdIom  = `/usr/bin/pgrep ^iom$`

	srosCfgTpl, _ = template.New("clab-sros-default-config").Funcs(clabutils.CreateFuncs()).
			Parse(srosConfigCmdsTpl)

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

	for k, v := range slotA.Env {
		vars[k] = v
	}

	return vars, nil
}

// Pre Deploy func for SR-SIM kind.
func (n *sros) PreDeploy(_ context.Context, params *clabnodes.PreDeployParams) error {
	log.Debug("Running pre-deploy")
	// store the preDeployParams
	n.preDeployParams = params

	n.InterfaceHelp = InterfaceHelp

	// store provided pubkeys
	n.sshPubKeys = params.SSHPubKeys

	if strings.HasPrefix(n.Cfg.NetworkMode, "container:") {
		n.Cfg.ExtraHosts = nil
	}

	// Create files/dir structure for standalone nodes or distributed CPM nodes
	if n.isStandaloneNode() || (n.isDistributedCardNode() && n.isCPM("")) {
		// generate the certificate
		if *n.Cfg.Certificate.Issue {
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

	// Populate /etc/hosts for service discovery on mgmt interface
	if err := n.populateHosts(ctx, params.Nodes); err != nil {
		log.Warn("Unable to populate hosts list", "node", n.Cfg.ShortName, "err", err)
	}

	var err error
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
				err = n.tlsCertBootstrap(ctx, addr)
				if err != nil {
					return fmt.Errorf(
						"TLS cert/key bootstrap to node %q failed: %w",
						n.Cfg.LongName,
						err,
					)
				}
				log.Infof(
					"Completed NETCONF bootstrap for gRPC-TLS profile on node %s",
					n.Cfg.ShortName,
				)
			}
			err = n.saveConfigWithAddr(ctx, addr)
			if err != nil {
				return fmt.Errorf("save config to node %q, failed: %w", n.Cfg.LongName, err)
			}
			return nil
		}

		// Wait before next check
		select {
		case <-ctx.Done():
			return ctx.Err()
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

func (n *sros) setupComponentNodes() error {
	if !n.isDistributedBaseNode() {
		return nil
	}

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

		// set the type var if type is set
		if c.Type != "" {
			componentConfig.Env[envNokiaSrosCard] = c.Type
		}

		if c.SFM != "" {
			componentConfig.Env[envNokiaSrosSFM] = c.SFM
		}

		if len(c.XIOM) > 0 {
			for _, x := range c.XIOM {
				key := fmt.Sprintf("%s_X%d", envNokiaSrosXIOM, x.Slot)
				componentConfig.Env[key] = x.Type
				// add the nested MDA
				for _, m := range x.MDA {
					key := fmt.Sprintf("%s_X%d_%d", envNokiaSrosMDA, x.Slot, m.Slot)
					componentConfig.Env[key] = m.Type
				}
			}
		}

		if len(c.MDA) > 0 {
			for _, m := range c.MDA {
				key := fmt.Sprintf("%s_%d", envNokiaSrosMDA, m.Slot)
				componentConfig.Env[key] = m.Type
			}
		}

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
		}

		// store the node in the componentNodes
		n.componentNodes = append(n.componentNodes, componentNode)
	}
	return nil
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

	// adjust general node to be represented as the cpm node
	n.Cfg.ShortName = n.calcComponentName(n.Cfg.ShortName, cpmSlot)
	n.Cfg.LongName = n.calcComponentName(n.Cfg.LongName, cpmSlot)
	n.Cfg.Fqdn = n.calcComponentFqdn(cpmSlot)
	n.renameDone = true

	cpmNode, err := n.cpmNode()
	if err != nil {
		return err
	}

	// adjust also the mgmt IP addresses of the general node
	contList, err := cpmNode.GetContainers(ctx)
	if err != nil {
		return err
	}
	n.Cfg.MgmtIPv4Address = contList[0].GetContainerIPv4()
	n.Cfg.MgmtIPv6Address = contList[0].GetContainerIPv6()

	return nil
}

// isDistributedCard checks if the slot variable is set, hence it is an instance (slot) of a
// distributed setup.
func (n *sros) isDistributedCardNode() bool {
	_, exists := n.Cfg.Env[envNokiaSrosSlot]
	// is distributed if components is > 1 and the slot var exists.
	return exists && len(n.Cfg.Components) == 0
}

// check if SR-SIM is distributed: `components` key is present.
func (n *sros) isDistributedBaseNode() bool {
	return len(n.Cfg.Components) > 1
}

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
func (n *sros) cpmSlot() (string, error) {
	slot := ""

search:
	for _, comp := range n.Cfg.Components {
		switch comp.Slot {
		case slotAName:
			slot = slotAName
			// now we can break because the slot A is preferred, does not matter if B also exists
			break search
		case slotBName:
			slot = slotBName
			// we continue searching, because we prefer slot A
		}
	}
	if slot == "" {
		return "", fmt.Errorf("node %s: unable to determine default slot", n.GetShortName())
	}
	return slot, nil
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

// checkComponentSlotsConfig check the sros component slot config for validaity.
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
		// addd to component names map
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

// Func that handles the config generation for the SR-SIM kind.
func (n *sros) createSROSConfigFiles() error {
	// generate a startup config file
	// if the node has a `startup-config:` statement, the file specified in that section
	// will be used as a template in GenerateConfig()

	var cfgTemplate string
	var err error

	// Path pointing to the target config file under configCf3 dir
	cf3CfgFile := filepath.Join(
		n.Cfg.LabDir,
		n.Cfg.Env[envNokiaSrosSlot],
		configCf3,
		startupCfgName,
	)
	isPartial := isPartialConfigFile(n.Cfg.StartupConfig)

	// generate config and use that to boot node
	log.Debug("Reading startup-config", "node", n.Cfg.ShortName, "startup-config",
		n.Cfg.StartupConfig, "isPartial", isPartial)
	if n.Cfg.StartupConfig != "" && !isPartial { // User provides startup config
		c, err := os.ReadFile(n.Cfg.StartupConfig)
		if err != nil {
			return err
		}

		cBuf, err := clabutils.SubstituteEnvsAndTemplate(bytes.NewReader(c), n.Cfg)
		if err != nil {
			return err
		}

		cfgTemplate = cBuf.String()
	} else { // Cases default config or default+partial
		err = n.addDefaultConfig()
		if err != nil {
			return err
		}
		// Adds partial config if present
		err = n.addPartialConfig()
		if err != nil {
			return err
		}
		cfgTemplate = string(n.startupCliCfg)
	}

	if cfgTemplate == "" {
		log.Debug(
			"configuration template is empty, skipping startup config file generation",
			"node",
			n.Cfg.ShortName,
		)
		return nil
	}

	err = n.GenerateConfig(cf3CfgFile, cfgTemplate)
	if err != nil {
		return fmt.Errorf("failed to generate config for node %q: %v", n.Cfg.ShortName, err)
	} else {
		return nil
	}
}

// SlotIsInteger checks if the slot string represents a valid integer.
func SlotIsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// Check if a container is a CPM.
func (n *sros) isCPM(cpm string) bool {
	// Check if container is a linecard
	if _, exists := n.Cfg.Env[envNokiaSrosSlot]; exists &&
		SlotIsInteger(n.Cfg.Env[envNokiaSrosSlot]) {
		return false
	}
	// check if container is the CPM given by the string cpm
	if cpm != "" {
		if _, exists := n.Cfg.Env[envNokiaSrosSlot]; exists &&
			!SlotIsInteger(n.Cfg.Env[envNokiaSrosSlot]) &&
			!strings.EqualFold(n.Cfg.Env[envNokiaSrosSlot], cpm) {
			return false
		}
	}
	// None of the previous conditions are meet
	return true
}

// addDefaultConfig adds sros default configuration such as tls certs, gnmi/json-rpc, login-banner,
// ssh keys.
func (n *sros) addDefaultConfig() error {
	b, err := n.banner()
	if err != nil {
		return err
	}

	componentConfig := ""

	// Generate component configuration IF no startup-config defined.
	if n.Cfg.StartupConfig == "" {
		componentConfig = n.generateComponentConfig()
	} else {
		log.Debugf("SR-SIM node %q has startup-config defined, skipping component config gen", n.Cfg.LongName)
	}

	// tplData holds data used in templating of the default config snippet
	tplData := srosTemplateData{
		Name:            n.Cfg.ShortName,
		TLSKey:          n.Cfg.TLSKey,
		TLSCert:         n.Cfg.TLSCert,
		TLSAnchor:       n.Cfg.TLSAnchor,
		Banner:          b,
		IFaces:          map[string]tplIFace{},
		MgmtMTU:         0,
		MgmtIPMTU:       0,
		SystemConfig:    systemCfg,
		SNMPConfig:      snmpv2Config,
		GRPCConfig:      grpcConfig,
		NetconfConfig:   netconfConfig,
		LoggingConfig:   loggingConfig,
		SSHConfig:       sshConfig,
		NodeType:        strings.ToLower(n.Cfg.NodeType),
		ComponentConfig: componentConfig,
	}
	if strings.Contains(tplData.NodeType, "ixr-") {
		tplData.GRPCConfig = grpcConfigIXR
		tplData.SystemConfig = systemCfgIXR
	}
	if !*n.Cfg.Certificate.Issue {
		log.Debugf("Using insecure cert configuration for node %s, found certificate.issue flag %v",
			n.Cfg.ShortName, *n.Cfg.Certificate.Issue)
		tplData.GRPCConfig = grpcConfigInsecure
		if strings.Contains(tplData.NodeType, "ixr-") {
			tplData.GRPCConfig = grpcConfigIXRInsecure
		}
	}

	if n.Config().DNS != nil {
		tplData.DNSServers = append(tplData.DNSServers, n.Config().DNS.Servers...)
	}

	n.prepareSSHPubKeys(&tplData)

	tplData.MgmtIPMTU = n.Runtime.Mgmt().MTU
	buf := new(bytes.Buffer)
	err = srosCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}
	if buf.Len() == 0 {
		log.Warn(
			"Buffer empty, template parsing error",
			"node",
			n.Cfg.ShortName,
			"template",
			srosCfgTpl.Name(),
		)
	} else {
		log.Debug("Additional default config parsed", "node", n.Cfg.ShortName, "template", srosCfgTpl.Name())
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
		if isPartialConfigFile(n.Cfg.StartupConfig) && n.isCPM("") {
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

	return cnts, err
}

// populateHosts adds container hostnames for other nodes of a lab to SR Linux /etc/hosts file
// to mitigate the fact that srlinux uses non default netns for management and thus
// can't leverage docker DNS service.
func (n *sros) populateHosts(ctx context.Context, nodes map[string]clabnodes.Node) error {
	hosts, err := n.Runtime.GetHostsPath(ctx, n.Cfg.LongName)
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

func (n *sros) SaveConfig(ctx context.Context) error {
	fqdn := ""
	switch {
	case n.isStandaloneNode():
		// check if it is a cpm node. return without error if not
		if !n.isCPM("") {
			return nil
		}
		// if it is a standalone node use the fqdn
		fqdn = n.Cfg.Fqdn
	case n.isDistributedBaseNode():
		// if it is the
		cmpNode, err := n.cpmNode()
		if err != nil {
			return err
		}
		// delegate to cpm node
		return cmpNode.SaveConfig(ctx)
	case n.isDistributedCardNode():
		// check if it is a cpm node. return without error if not
		if !n.isCPM("") {
			return nil
		}
		fqdn = n.Cfg.LongName
	}
	return n.saveConfigWithAddr(ctx, fqdn)
}

// saveConfigWithAddr will use the addr string to try to save the config of the node.
func (n *sros) saveConfigWithAddr(_ context.Context, addr string) error {
	err := clabnetconf.SaveRunningConfig(fmt.Sprintf("[%s]", addr),
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		scrapliPlatformName,
	)
	if err != nil {
		return err
	}

	log.Info("Saved running configuration", "node", n.Cfg.ShortName, "addr", addr)

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

// TLS bootstrap via NETCONF to enable secure gRPC:
//   - `imports cf3:\node.key` in PEM format as `cf3:\system-pki\node.key“ (encrypted DER)
//   - `imports cf3:\node.crt` in PEM format as `cf3:\system-pki\node.crt“ (encrypted DER)
//   - administratively enables TLS profile `grpc-tls-certs`
func (n *sros) tlsCertBootstrap(_ context.Context, addr string) error {
	operations := []clabnetconf.Operation{
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.RPC(opoptions.WithFilter(buildPKIImportXML(
				fmt.Sprintf("cf3:/%s", tlsKeyFile), tlsKeyFile, "key")))
		},
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.RPC(opoptions.WithFilter(buildPKIImportXML(
				fmt.Sprintf("cf3:/%s", tlsCertFile), tlsCertFile, "certificate")))
		},
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.EditConfig("candidate", buildTLSProfileXML())
		},
		func(d *netconf.Driver) (*response.NetconfResponse, error) {
			return d.Commit()
		},
	}

	err := clabnetconf.MultiExec(
		fmt.Sprintf("[%s]", addr),
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		operations,
	)
	return err
}

// isPartialConfigFile returns true if the config file name contains .partial substring.
func isPartialConfigFile(c string) bool {
	return strings.Contains(strings.ToUpper(c), ".PARTIAL")
}

func (n *sros) IsHealthy(_ context.Context) (bool, error) {
	if !n.isCPM("") {
		return true, fmt.Errorf("node %q is not a CPM, healthcheck has no effect", n.Cfg.LongName)
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

// MgmtIPAddr returns ipv4 or ipv6 management IP address of the node.
// It returns an error if neither is set in the node config.
func (n *sros) MgmtIPAddr() (string, error) {
	switch {
	case n.Cfg.MgmtIPv6Address != "":
		return n.Cfg.MgmtIPv6Address, nil
	case n.Cfg.MgmtIPv4Address != "":
		return n.Cfg.MgmtIPv4Address, nil
	}
	return n.Cfg.LongName, fmt.Errorf(
		"no management IP address (IPv4 or IPv6) configured for node %q",
		n.Cfg.LongName,
	)
}

// generateComponentConfig generates SR OS configuration
// for explicitly defined components using components nodeConfig.
func (n *sros) generateComponentConfig() string {
	if _, exists := n.Cfg.Env[envDisableComponentConfigGen]; exists {
		return ""
	}

	if n.isStandaloneNode() {
		// skip integrated for now
		return ""
	}

	components := n.rootComponents

	if len(components) == 0 {
		return ""
	}

	var config strings.Builder

	for _, component := range components {
		slot := strings.ToUpper(component.Slot)

		if slot == "A" || slot == "B" {
			// cpm -> skip
			continue
		}

		if component.Type != "" {
			config.WriteString(
				fmt.Sprintf(
					"/configure card %s card-type %s admin-state enable\n",
					slot,
					component.Type,
				),
			)
		} else {
			// not all types may have preprovisioned card
			// so the following configs will fail if card
			// doesn't have valid type explicitly set.
			log.Warnf("SR-SIM node %q, has no type set for component in slot %q, skipping component SR-OS config generation.", n.Cfg.ShortName, slot)
			continue
		}

		if component.SFM != "" {
			config.WriteString(
				fmt.Sprintf(
					"/configure sfm %s sfm-type %s admin-state enable\n",
					slot,
					component.SFM,
				),
			)
		}

		for _, xiom := range component.XIOM {
			if xiom.Type != "" {
				config.WriteString(
					fmt.Sprintf(
						"/configure card %s xiom x%d xiom-type %s admin-state enable\n",
						slot,
						xiom.Slot,
						xiom.Type,
					),
				)
			}

			for _, mda := range xiom.MDA {
				if mda.Type != "" {
					config.WriteString(
						fmt.Sprintf(
							"/configure card %s xiom x%d mda %d mda-type %s admin-state enable\n",
							slot,
							xiom.Slot,
							mda.Slot,
							mda.Type,
						),
					)
				}
			}
		}

		for _, mda := range component.MDA {
			if mda.Type != "" {
				config.WriteString(
					fmt.Sprintf(
						"/configure card %s mda %d mda-type %s admin-state enable\n",
						slot,
						mda.Slot,
						mda.Type,
					),
				)
			}
		}
	}

	config.WriteString(n.generatePowerConfig())

	return config.String()
}

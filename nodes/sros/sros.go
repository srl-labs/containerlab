// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brunoga/deep"
	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/opoptions"

	"github.com/scrapli/scrapligo/response"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
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

	readyTimeout   = time.Minute * 1 // max wait time for node to boot
	tcpDialTimeout = time.Second * 1

	generateable     = true
	generateIfFormat = "%d/%d/%d"

	slotAName = "A"
	slotBName = "B"

	standaloneSlotName = slotAName

	retryTimer = 1 * time.Second

	scrapliPlatformName        = "nokia_sros"
	scrapliPlatformNameClassic = "nokia_sros_classic"
	configCf3                  = "config/cf3"
	configCf2                  = "config/cf2"
	configCf1                  = "config/cf1"
	startupCfgName             = "config.cfg"
	tlsKeyFile                 = "node.key"
	tlsCertFile                = "node.crt"
	tlsCertProfileName         = "clab-grpc-certs"

	// SR-SIM ENV VARS.
	envNokiaSrosSlot          = "NOKIA_SROS_SLOT"
	envNokiaSrosChassis       = "NOKIA_SROS_CHASSIS"
	envNokiaSrosSystemBaseMac = "NOKIA_SROS_SYSTEM_BASE_MAC"
	envNokiaSrosCard          = "NOKIA_SROS_CARD"
	envNokiaSrosSFM           = "NOKIA_SROS_SFM"
	envNokiaSrosXIOM          = "NOKIA_SROS_XIOM"
	envNokiaSrosMDA           = "NOKIA_SROS_MDA"
	envSrosIPv4Active         = "NOKIA_SROS_ADDRESS_IPV4_ACTIVE"
	envSrosIPv6Active         = "NOKIA_SROS_ADDRESS_IPV6_ACTIVE"
	envSrosStaticRoutePrefix  = "NOKIA_SROS_STATIC_ROUTE_"

	srosMinorError     = "MINOR:"
	srosCriticalError  = "CRITICAL:"
	srosRejectedCfgMsg = "Configuration load failed - using default configuration"
)

var (
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
	).WithKindSpecificConfig(kindSpecificConfig)

	r.Register(kindNames, func() clabnodes.Node {
		return new(sros)
	}, nrea)
}

// KindSpecificConfig is the nokia_srsim kind-specific config, set as keys on the node definition.
type KindSpecificConfig struct {
	// ConfigMode is the SR OS configuration mode: model-driven (default), classic or mixed.
	ConfigMode ConfigMode `yaml:"config-mode,omitempty" json:"config-mode,omitempty"`
	// GenComponentConfig generates the configuration of the node's components. Defaults to true.
	GenComponentConfig bool         `yaml:"gen-component-config" json:"gen-component-config"`
	SFM                string       `yaml:"sfm,omitempty" json:"sfm,omitempty"`
	Components         []*Component `yaml:"components,omitempty" json:"components,omitempty"`
}

// SetDefaults implements clabnodes.KindSpecificConfigDefaulter.
func (c *KindSpecificConfig) SetDefaults() { c.GenComponentConfig = true }

// equalIgnoringComponentOrder reports whether c and o are equal, comparing components by slot
// regardless of their order and the case of the slot.
func (c *KindSpecificConfig) equalIgnoringComponentOrder(o *KindSpecificConfig) bool {
	a, b := *c, *o
	a.Components, b.Components = nil, nil

	return reflect.DeepEqual(a, b) &&
		reflect.DeepEqual(componentsBySlot(c.Components), componentsBySlot(o.Components))
}

var kindSpecificConfig clabnodes.KindSpecificConfigSpec[KindSpecificConfig]

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
	rootComponents []*Component
	// store the longname with cpm suffix
	cpmContainerName string
	// for component nodes, store base nodes
	baseShortName string
	baseLongName  string
	// rootCtrName is the container that owns the netns.
	//  - for components based: the internal netns container.
	//  - for network mode based: parsed from network-mode.
	rootCtrName string

	preDeployParams *clabnodes.PreDeployParams

	netnsNode clabnodes.Node
}

func (*sros) LinkApplyMode(context.Context) clabnodes.LinkApplyMode {
	return clabnodes.LinkApplyModeLive
}

// Init Function for SR-SIM kind.
func (n *sros) kindSpecificCfg() *KindSpecificConfig { return kindSpecificConfig.Of(n.Cfg) }

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

	if sfm := n.kindSpecificCfg().SFM; sfm != "" {
		if n.Cfg.Env == nil {
			n.Cfg.Env = map[string]string{}
		}

		n.Cfg.Env[envNokiaSrosSFM] = sfm
	}

	maps.Copy(n.Cfg.Sysctls, srosSysctl)

	// make sure we always have uppercase slot definition
	for _, c := range n.kindSpecificCfg().Components {
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
		n.Cfg.Certificate.Issue = new(false)
	}
	if n.isStandaloneNode() {
		log.Debugf(
			"%q is standalone node. %v",
			n.Cfg.ShortName,
			len(n.kindSpecificCfg().Components),
		)

		vars, err := n.setupStandaloneComponents()
		if err != nil {
			return err
		}

		// n.Cfg.Env overrides component vars, overrides default srosEnv
		n.Cfg.Env = clabutils.MergeStringMaps(srosEnv, vars, n.Cfg.Env)
		log.Debug("Merged env file", "env", fmt.Sprintf("%+v", n.Cfg.Env), "node", n.Cfg.ShortName)
	} else {
		log.Debugf(
			"%q is distributed node. %v",
			n.Cfg.ShortName,
			len(n.kindSpecificCfg().Components),
		)

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

	if len(n.kindSpecificCfg().Components) == 0 {
		return nil, nil
	}
	if len(n.kindSpecificCfg().Components) > 1 {
		return nil, fmt.Errorf(
			"expected at most one component override for standalone SR-SIM node %q, "+
				"or one component per slot %s for redundant type %q",
			n.Cfg.ShortName,
			strings.Join(integratedSrosAllowedSlots(n.Cfg.NodeType), "/"),
			n.Cfg.NodeType,
		)
	}

	slotA := n.kindSpecificCfg().Components[0]

	slotName := strings.ToUpper(strings.TrimSpace(slotA.Slot))
	// single undefined slot is implicitly set to A
	if slotName == "" {
		slotName = standaloneSlotName
	}

	if !integratedSrosSlotAllowed(n.Cfg.NodeType, slotName) {
		return nil, fmt.Errorf(
			"expected no slot, or slot %s for components of standalone SR-SIM node %q",
			strings.Join(integratedSrosAllowedSlots(n.Cfg.NodeType), "/"),
			n.Cfg.ShortName,
		)
	}
	vars[envNokiaSrosSlot] = slotName

	setComponentEnvVars(vars, slotA)

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

		return n.createSROSFiles(ctx)
	}
	return nil
}

func (n *sros) DeployEndpoints(ctx context.Context) error {
	if err := n.DefaultNode.DeployEndpoints(ctx); err != nil {
		return err
	}

	return n.PostDeployEndpoints(ctx)
}

// PostDeployEndpoints runs SR-SIM endpoint fixups after dataplane links exist.
func (n *sros) PostDeployEndpoints(ctx context.Context) error {
	if n.Runtime.Mgmt().Driver == clabtypes.MgmtDriverMacvlan {
		return nil
	}
	// Disable TX checksum offload on the host NS veth for the mgmt interface.
	var peerIfIndex int
	err := n.ExecFunction(ctx, clabutils.VethPeerIndex("eth0", &peerIfIndex))
	if err != nil {
		log.Warn("Failed to get veth peer index for SR-SIM mgmt interface",
			"node", n.Cfg.ShortName,
			"error", err)
		return nil
	}

	if err := clabutils.DisableTxOffloadByIndex(peerIfIndex); err != nil {
		log.Warn("Failed to disable TX checksum offload on SR-SIM mgmt host veth",
			"node", n.Cfg.ShortName,
			"error", err)
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

	if err := n.RequireMgmtReachable(); err != nil {
		return err
	}

	// Execute SaveConfig after boot. This code should only run on active CPM
	deadline := time.Now().Add(readyTimeout)
	var lastHealthErr error
	for time.Now().Before(deadline) {
		// Check if context is canceled
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context canceled: %w", err)
		}

		isHealthy, err := n.IsHealthy(ctx)
		if err != nil {
			lastHealthErr = err
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

	if lastHealthErr != nil {
		return fmt.Errorf("node %q did not become healthy before timeout: %w",
			n.Cfg.LongName, lastHealthErr)
	}

	return fmt.Errorf("node %q did not become healthy before timeout", n.Cfg.LongName)
}

// Delete func for SR-SIM kind.
func (n *sros) Delete(ctx context.Context) error {
	// if not distributed, follow default node implementation
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.DefaultNode.Delete(ctx)
	}

	var errs []error

	// Delete all the component containers
	for _, componentNode := range n.componentNodes {
		err := componentNode.Delete(ctx)
		if err != nil {
			log.Warn(err)
			errs = append(errs, err)
		}
	}

	errs = append(errs, n.DefaultNode.Delete(ctx), n.netnsNode.Delete(ctx))

	return errors.Join(errs...)
}

func (n *sros) setupComponentNodes() error {
	if !n.isDistributedBaseNode() {
		return nil
	}

	n.netnsNode = new(namespaceNode)
	if err := n.netnsNode.Init(
		n.newNetnsConfig(),
		clabnodes.WithRuntime(n.GetRuntime()),
	); err != nil {
		return err
	}
	rootCtrName := n.netnsNode.Config().LongName

	// loop through the components, creating them
	for _, c := range n.kindSpecificCfg().Components {
		// instantiate a new nokia_srsim instance
		srosNode := new(sros)
		componentNode := clabnodes.Node(srosNode)

		// copy the original node's NodeConfig to the component
		componentConfig, err := deep.Copy(n.Cfg)
		if err != nil {
			return fmt.Errorf("failed to deep copy node config for component: %w", err)
		}

		// component nodes join to the nents pause container
		componentConfig.NetworkMode = fmt.Sprintf("container:%s", n.netnsNode.GetShortName())

		// adjust the config values from the original node
		componentConfig.ShortName = n.calcComponentName(componentConfig.ShortName, c.Slot)
		componentConfig.LongName = n.calcComponentName(componentConfig.LongName, c.Slot)
		componentConfig.NodeType = n.Cfg.NodeType
		kindSpecificConfig.Of(componentConfig).Components = nil
		componentConfig.Fqdn = n.calcComponentFqdn(c.Slot)
		componentConfig.DNS = nil
		componentConfig.PortBindings = nil
		componentConfig.PortSet = nil

		// add the component env to the componentConfig env
		for k, v := range c.Env {
			componentConfig.Env[k] = v
		}

		// set component environment variables
		setComponentEnvVars(componentConfig.Env, c)
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
		srosNode.rootComponents = n.kindSpecificCfg().Components
		// store base node name
		srosNode.baseShortName = n.Cfg.ShortName
		srosNode.baseLongName = n.Cfg.LongName
		srosNode.rootCtrName = rootCtrName

		// store the node in the componentNodes
		n.componentNodes = append(n.componentNodes, componentNode)
	}
	return nil
}

// deployFabric deploys the distributed SR-SIM when the `components` key is present.
func (n *sros) deployFabric(ctx context.Context, deployParams *clabnodes.DeployParams) error {
	netnsConfig := n.netnsNode.Config()

	netnsConfig.MgmtIPv4Address = n.Cfg.MgmtIPv4Address
	netnsConfig.MgmtIPv4PrefixLength = n.Cfg.MgmtIPv4PrefixLength
	netnsConfig.MgmtIPv4Gateway = n.Cfg.MgmtIPv4Gateway
	netnsConfig.MgmtIPv6Address = n.Cfg.MgmtIPv6Address
	netnsConfig.MgmtIPv6PrefixLength = n.Cfg.MgmtIPv6PrefixLength
	netnsConfig.MgmtIPv6Gateway = n.Cfg.MgmtIPv6Gateway

	if err := n.netnsNode.Deploy(ctx, deployParams); err != nil {
		return fmt.Errorf("deploy network namespace node %q: %w", n.netnsNode.GetShortName(), err)
	}

	ips, err := n.distNodeMgmtIPs()
	if err != nil {
		return err
	}

	n.setComponentMgmtEnv(ips)

	// loop through the components, creating them
	for _, c := range n.componentNodes {
		if err := c.PreDeploy(ctx, n.preDeployParams); err != nil {
			return fmt.Errorf("pre-deploy for component node %q: %w", c.GetShortName(), err)
		}
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
	n.Cfg.MgmtIPv4Address = ips.IPv4
	n.Cfg.MgmtIPv6Address = ips.IPv6

	return nil
}

// set env vars for the CPMs so that they are aware of mgmt info.
func (n *sros) setComponentMgmtEnv(ips MgmtIP) {
	for _, component := range n.componentNodes {
		cfg := component.Config()
		slot := strings.ToUpper(cfg.Env[envNokiaSrosSlot])

		if slot != slotAName && slot != slotBName {
			continue
		}

		env := cfg.Env
		if intf := env["NOKIA_SROS_MGMT_IF"]; intf != "" && intf != "eth0" {
			continue
		}
		if env[envSrosIPv4Active] != "" || env[envSrosIPv6Active] != "" {
			continue
		}

		// SKIP if the user defines this env var
		explicitRoutes := false
		for key := range env {
			if strings.HasPrefix(key, envSrosStaticRoutePrefix) {
				explicitRoutes = true
				break
			}
		}
		if explicitRoutes {
			continue
		}

		if ips.IPv4 != "" {
			env[envSrosIPv4Active] = fmt.Sprintf("%s/%d", ips.IPv4, ips.IPv4pLen)
			if ips.IPv4Gw != "" {
				env[envSrosStaticRoutePrefix+"1"] = "0.0.0.0/0@" + ips.IPv4Gw
			}
		}

		if ips.IPv6 != "" {
			env[envSrosIPv6Active] = fmt.Sprintf("%s/%d", ips.IPv6, ips.IPv6pLen)
			if ips.IPv6Gw != "" {
				env[envSrosStaticRoutePrefix+"2"] = "::/0@" + ips.IPv6Gw
			}
		}
	}
}

// isDistributedCard checks if the slot variable is set, hence it is an instance (slot) of a
// distributed setup.
// isDistributedCardNode returns true if this node is a card (linecard/IOM)
// in a distributed SR-SIM deployment. A distributed card has a slot assignment
// but no components of its own.
func (n *sros) isDistributedCardNode() bool {
	_, exists := n.Cfg.Env[envNokiaSrosSlot]
	return exists && len(n.kindSpecificCfg().Components) == 0
}

// isDistributedBaseNode returns true if this is the base node of a distributed
// SR-SIM deployment. The base node orchestrates multiple component nodes.
func (n *sros) isDistributedBaseNode() bool {
	if isIntegratedSrosNodeType(n.Cfg.NodeType) {
		return isRedundantIntegratedSrosComponents(n.Cfg.NodeType, n.kindSpecificCfg().Components)
	}
	return len(n.kindSpecificCfg().Components) > 1
}

// isStandaloneNode returns true if this is a standalone (non-distributed) SR-SIM node.
func (n *sros) isStandaloneNode() bool {
	return !n.isDistributedBaseNode() && !n.isDistributedCardNode()
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
	if fqdnDotIndex < 0 {
		return fmt.Sprintf("%s-%s", n.Cfg.Fqdn, strings.ToLower(slot))
	}
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
	for _, comp := range n.kindSpecificCfg().Components {
		if comp.Slot == slotAName {
			return slotAName, nil
		}
	}
	// Check for slot B as fallback
	for _, comp := range n.kindSpecificCfg().Components {
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
func (n *sros) createSROSFiles(ctx context.Context) error {
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
		// check for the netns holder as to never orphan it.
		if n.netnsNode != nil {
			return n.netnsNode.GetContainers(ctx)
		}
		return nil, fmt.Errorf("node: %s. %w", n.GetContainerName(),
			clabnodes.ErrContainersNotFound)
	}

	// Forge the IP address to be the actual IP of mgmt
	// because the CPM A might not own the netns & mgmt IP
	if len(n.kindSpecificCfg().Components) > 0 {
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
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
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
		n.kindSpecificCfg().ConfigMode,
	)

	return nil
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
		n.Cfg.Credentials.Username,
		n.Cfg.Credentials.Password,
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
	return CheckPortWithRetry(addr, 830, readyTimeout, int(readyTimeout/retryTimer), retryTimer)
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
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	deadline := time.Now().Add(timeout)

	for i := range maxRetries {
		if i > 0 {
			if remaining := time.Until(deadline); remaining <= 0 {
				break
			} else if retryDelay < remaining {
				time.Sleep(retryDelay)
			} else {
				time.Sleep(remaining)
			}
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		dialTimeout := tcpDialTimeout
		if remaining < dialTimeout {
			dialTimeout = remaining
		}

		conn, err := net.DialTimeout("tcp", address, dialTimeout)
		if err == nil {
			conn.Close()
			return true, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out checking tcp port %s", address)
	}

	return false, lastErr
}

// MgmtIP represents the management IPv4/v6 addresses of a node.
type MgmtIP struct {
	IPv4        string
	IPv4pLen    int
	IPv4Gw      string
	IPv6        string
	IPv6pLen    int
	IPv6Gw      string
	ContainerID string
}

// distNodeMgmtIPs returns both ipv4 and ipv6 management IP address of a
// distributed node defined via components.
// It returns an error if neither is set in the node config.
func (n *sros) distNodeMgmtIPs() (MgmtIP, error) {
	ips := MgmtIP{}

	var containerName string
	if n.isDistributedBaseNode() {
		if n.netnsNode != nil {
			containerName = n.netnsNode.Config().LongName
		}
	} else {
		containerName = n.rootCtrName
	}

	if containerName == "" {
		return ips, nil
	}

	containers, err := n.Runtime.ListContainers(
		context.Background(),
		[]*clabtypes.GenericFilter{
			{
				FilterType: "name",
				Match:      containerName,
			},
		},
	)
	if err != nil {
		return MgmtIP{}, fmt.Errorf("unable to get container %q: %v", containerName, err)
	}

	for _, container := range containers {
		if container.NetworkSettings.IPv4addr != "" {
			ips.IPv4 = container.NetworkSettings.IPv4addr
			ips.IPv4pLen = container.NetworkSettings.IPv4pLen
			ips.IPv4Gw = container.NetworkSettings.IPv4Gw
			ips.ContainerID = container.ID
		}
		if container.NetworkSettings.IPv6addr != "" {
			ips.IPv6 = container.NetworkSettings.IPv6addr
			ips.IPv6pLen = container.NetworkSettings.IPv6pLen
			ips.IPv6Gw = container.NetworkSettings.IPv6Gw
			ips.ContainerID = container.ID
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
			log.Debug(
				"Skipping nokia_srsim image kind check: runtime does not support image inspection",
			)
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
	log.Warnf(
		"node %q: kind is nokia_srsim but the provided image does not have the correct labels; please use a valid SR-SIM container image or run a more recent version of the SR-SIM container image to suppress this warning",
		n.Cfg.ShortName,
	)
	return nil
}

func (n *sros) GetContainerStatus(ctx context.Context) clabruntime.ContainerStatus {
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.DefaultNode.GetContainerStatus(ctx)
	}

	var (
		hasRunning    bool
		hasPaused     bool
		hasRestarting bool
		hasCreated    bool
		hasStopped    bool
		hasRemoving   bool
	)

	for _, componentNode := range n.componentNodes {
		switch componentNode.GetContainerStatus(ctx) {
		case clabruntime.Running:
			hasRunning = true
		case clabruntime.Paused:
			hasPaused = true
		case clabruntime.Restarting:
			hasRestarting = true
		case clabruntime.Created:
			hasCreated = true
		case clabruntime.Stopped:
			hasStopped = true
		case clabruntime.Removing:
			hasRemoving = true
		}
	}

	switch {
	case hasRunning:
		return clabruntime.Running
	case hasPaused:
		return clabruntime.Paused
	case hasRestarting:
		return clabruntime.Restarting
	case hasCreated:
		return clabruntime.Created
	case hasStopped:
		return clabruntime.Stopped
	case hasRemoving:
		return clabruntime.Removing
	default:
		return clabruntime.NotFound
	}
}

func (n *sros) Start(ctx context.Context) error {
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		if err := n.DefaultNode.Start(ctx); err != nil {
			return err
		}
		return n.PostDeployEndpoints(ctx)
	}

	if err := n.ensureNetnsRunning(ctx); err != nil {
		return err
	}

	for _, c := range n.componentNodes {
		if _, err := n.Runtime.StartContainer(ctx, c.Config().LongName, c); err != nil {
			return fmt.Errorf("node %q component %q start error: %w",
				n.Cfg.ShortName, c.Config().ShortName, err)
		}
	}

	if err := n.RestoreEndpoints(ctx); err != nil {
		return err
	}
	return n.PostDeployEndpoints(ctx)
}

func (n *sros) Stop(ctx context.Context) error {
	if n.isStandaloneNode() || n.isDistributedCardNode() {
		return n.DefaultNode.Stop(ctx)
	}

	if n.GetContainerStatus(ctx) == clabruntime.Stopped {
		return nil
	}
	if err := n.ParkEndpoints(ctx); err != nil {
		return err
	}

	for _, c := range n.componentNodes {
		if err := n.Runtime.StopContainer(ctx, c.Config().LongName, n.StopSignal); err != nil {
			log.Warnf("node %q component %q stop error: %v",
				n.Cfg.ShortName, c.Config().ShortName, err)
		}
	}

	return nil
}

// ComputeDiff extends the default diff by ignoring component order and slot case, so that only
// a change to the set of components marks the kind-specific config as changed.
func (n *sros) ComputeDiff(oldCfg, newCfg *clabtypes.NodeConfig) *clabtypes.TopologyDiff {
	diff := n.DefaultNode.ComputeDiff(oldCfg, newCfg)

	if oldCfg == nil || newCfg == nil {
		return diff
	}

	oldKC, oldOK := oldCfg.KindSpecificConfig.(*KindSpecificConfig)
	newKC, newOK := newCfg.KindSpecificConfig.(*KindSpecificConfig)

	if oldOK && newOK && oldKC.equalIgnoringComponentOrder(newKC) {
		diff.Fields = slices.DeleteFunc(
			diff.Fields,
			func(f string) bool { return f == "KindSpecificConfig" },
		)
	}

	return diff
}

// DefaultLinkType returns the default link type for an SR-SIM node
// when the brief notation of a link definition is used.
// It returns veth-stitch for SR-SIM nodes to support pcap and netem features,
// see https://github.com/srl-labs/containerlab/pull/3270.
func (*sros) DefaultLinkType() clablinks.LinkType {
	return clablinks.LinkTypeVethStitch
}

// RestoreEndpoints retains normal lifecycle parking while restoring the logical
// node's symlink as well as the component symlinks created by the runtime.
func (n *sros) RestoreEndpoints(ctx context.Context) error {
	if err := n.DefaultNode.RestoreEndpoints(ctx); err != nil {
		return err
	}
	if !n.isDistributedBaseNode() {
		return nil
	}
	nsPath, err := n.GetNSPath(ctx)
	if err != nil {
		return err
	}
	return clabutils.LinkContainerNS(nsPath, n.Cfg.LongName)
}

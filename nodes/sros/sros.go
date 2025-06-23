// Copyright 2025 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sros

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"

	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	SrosDefaultType = "SR-1" // default sros node type

	readyTimeout = time.Minute * 5 // max wait time for node to boot

	generateable     = true
	generateIfFormat = "e%d-%d"

	retryTimer = time.Second

	// defaultCfgPath is a path to a file with default config that clab adds on top of the factory config.
	// Default config is a config that adds some basic configuration to the node, such as tls certs, gnmi/json-rpc, login-banner.
	// defaultCfgPath = "/tmp/clab-default-config"
	// // overlayCfgPath is a path to a file with additional config that clab adds on top of the default config.
	// // Partial config provided via startup-config parameter is an overlay config.
	// overlayCfgPath = "/tmp/clab-overlay-config"
)

var (

	// //go:embed topology/*
	// topologies embed.FS

	//go:embed sros_default_config.go.tpl
	srosConfigCmdsTpl string

	// additional config that clab adds on top of the factory config.
	scrapliPlatformName = "nokia_sros"

	kindNames  = []string{"sros", "nokia_srsim"}
	srosSysctl = map[string]string{
		"net.ipv4.ip_forward":                "0",
		"net.ipv6.conf.all.disable_ipv6":     "0",
		"net.ipv6.conf.default.disable_ipv6": "0",
		"net.ipv6.conf.all.accept_dad":       "0",
		"net.ipv6.conf.default.accept_dad":   "0",
		"net.ipv6.conf.all.autoconf":         "0",
		"net.ipv6.conf.default.autoconf":     "0",
		"net.ipv4.conf.all.rp_filter":        "0",
		"net.ipv4.conf.default.rp_filter":    "0",
	}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	// srosTypes = map[string]string{
	// 	"dms-1-24d": "DMS-1-24D.yml",
	// 	"ixr-10":    "IXR-10.yml",
	// 	"ixr-6":     "IXR-6.yml",
	// 	"ixr-e":     "IXR-e.yml",
	// 	"ixr-e2":    "IXR-e2.yml",
	// 	"ixr-e2c":   "IXR-e2c.yml",
	// 	"ixr-ec":    "IXR-ec.yml",
	// 	"ixr-r4":    "IXR-R4.yml",
	// 	"ixr-r6":    "IXR-R6.yml",
	// 	"ixr-r6d":   "IXR-R6d.yml",
	// 	"ixr-r6dl":  "IXR-R6dl.yml",
	// 	"ixr-s":     "IXR-s.yml",
	// 	"ixr-x":     "IXR-x.yml",
	// 	"ixr-x3":    "IXR-x3.yml",
	// 	"sar-1":     "SAR-1.yml",
	// 	"sar-hm":    "SAR-Hm.yml",
	// 	"sar-hmc":   "SAR-Hmc.yml",
	// 	"sr-1-24d":  "SR-1-24D.yml",
	// 	"sr-1-46s":  "SR-1-46S.yml",
	// 	"sr-1-48d":  "SR-1-48D.yml",
	// 	"sr-1-92s":  "SR-1-92S.yml",
	// 	"sr-1":      "SR-1.yml",
	// 	"sr-12":     "SR-12.yml",
	// 	"sr-12e":    "SR-12e.yml",
	// 	"sr-14s":    "SR-14s.yml",
	// 	"sr-1e":     "SR-1e.yml",
	// 	"sr-1se":    "SR-1se.yml",
	// 	"sr-1x-48d": "SR-1x-48D.yml",
	// 	"sr-1x-92s": "SR-1x-92S.yml",
	// 	"sr-2e":     "SR-2e.yml",
	// 	"sr-2s":     "SR-2s.yml",
	// 	"sr-2se":    "SR-2se.yml",
	// 	"sr-3e":     "SR-3e.yml",
	// 	"sr-7":      "SR-7.yml",
	// 	"sr-7s":     "SR-7s.yml",
	// 	"sr-a4":     "SR-a4.yml",
	// 	"vsr-i":     "VSR-I.yml",
	// 	"xrs-20":    "XRS-20.yml",
	// 	"xrs-20e":   "XRS-20e.yml",
	// }

	srosEnv = map[string]string{
		"SRSIM":                      "1",
		"NOKIA_SROS_CHASSIS":         SrosDefaultType,     // fillers to be override
		"NOKIA_SROS_SYSTEM_BASE_MAC": "fa:ac:ff:ff:10:00", // filler to be override
	}
	// srosTopology = srosTopo{}

	// saveCmd          = `/opt/srlinux/bin/sr_cli -d "tools system configuration save"`.
	readyCmd = `/usr/bin/pidof cpm`
	// // readyForConfigCmd checks the output of a file on srlinux which will be populated once the mgmt server is ready to accept config.
	// readyForConfigCmd = "cat /etc/opt/srlinux/devices/app_ephemeral.mgmt_server.ready_for_config".

	srosCfgTpl, _ = template.New("clab-sros-default-config").Funcs(utils.CreateFuncs()).
			Parse(srosConfigCmdsTpl)

	requiredKernelVersion = &utils.KernelVersion{
		Major:    4,
		Minor:    10,
		Revision: 0,
	}

	InterfaceRegexp  = regexp.MustCompile(`ethernet-(?P<linecard>\d+)/(?P<port>\d+)(?:/(?P<channel>\d+))?`)
	InterfaceRegexp2 = regexp.MustCompile(`^(?:e(?P<card>\d+)-(?:x(?P<xiom>\d+)-)?(?P<mda>\d+)(?:-c(?P<connector>\d+))?-(?P<port>\d+)|eth(?P<mgmtPort>\d+)|iom-(?P<iom>\d+)|(fab-)(?P<fabricPort>\d+|[ab]))$`)
	InterfaceHelp    = `The format of the interface name need to be one of:
      e1-2-3       -> card 1, mda 2, port 3
      e1-2-c3-4    -> card 1, mda 2, connector 3, port 4
      e1-x2-3-4    -> card 1, xiom 2, mda 3, port 4
      e1-x2-3-c4-5 -> card 1, xiom 2, mda 3, connector 4, port 5
	  eth0/eth1, for management interfaces of CPM-A/CPM-B
	  iom-[1-9] for iom management interfaces
	  fab[1-9] for fabric interfaces`
	CardRegexp = regexp.MustCompile(`^(?i)(?:iom-|lc-)?([1-9])$`)
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformOpts := &nodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
	}

	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformOpts)

	r.Register(kindNames, func() nodes.Node {
		return new(sros)
	}, nrea)
}

type sros struct {
	nodes.DefaultNode
	// startup-config passed as a path to a file with CLI instructions will be read into this byte slice
	startupCliCfg []byte

	// Params provided in Pre-Deploy, that sros uses in Post-Deploy phase
	// to generate certificates
	cert         *cert.Cert
	topologyName string
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// software version SR-OS x node runs
	swVersion *SrosVersion
}

// type srosTopo struct {
// 	Cpm  map[string][]map[string]string `yaml:"chassis_configuration"`
// 	Slot map[string][]map[string]string `yaml:"slot_configuration"`
// }

func (n *sros) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.SSSE3 = true
	n.HostRequirements.MinVCPU = 4
	n.HostRequirements.MinVCPUFailAction = types.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 8
	n.HostRequirements.MinAvailMemoryGbFailAction = types.FailBehaviourLog

	n.Cfg = cfg

	// force cert creation for sros nodes as they by make use of tls certificate in the default config
	n.Cfg.Certificate.Issue = utils.Pointer(true)

	for _, o := range opts {
		o(n)
	}

	if n.Cfg.NodeType == "" {
		n.Cfg.NodeType = SrosDefaultType
	}

	// n.srosNodeTypeToLower()
	// if _, found := srosTypes[n.Cfg.NodeType]; !found {
	// 	keys := make([]string, 0, len(srosTypes))
	// 	for key := range srosTypes {
	// 		keys = append(keys, key)
	// 	}
	// 	return fmt.Errorf("wrong node type '%s' doesn't exist. Should be any of %s",
	// 		n.Cfg.NodeType, strings.Join(keys, ", "))
	// }

	if n.Cfg.Cmd == "" {
		// set default Cmd if it was not provided by a user
		// the additional touch is needed to support non docker runtimes
		n.Cfg.Cmd = ""
	}

	// if user was not initialized to a value, use root
	if n.Cfg.User == "" {
		n.Cfg.User = "0:0"
	}

	maps.Copy(n.Cfg.Sysctls, srosSysctl)

	// for k, v := range srosSysctl {
	// 	n.Cfg.Sysctls[k] = v
	// }

	if n.Cfg.License != "" {
		// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
		n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(
			filepath.Join(n.Cfg.LabDir, "license.key"), ":/nokia/license/license.txt:ro"))
	}

	// mount config directory
	cfgPath := filepath.Join(n.Cfg.LabDir, "config")
	log.Debugf("cfgPath: %s", cfgPath)
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(cfgPath, ":/nokia/config/:rw"))
	log.Debugf("n.Cfg.Binds: %+v", n.Cfg.Binds)

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (n *sros) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	log.Debug("Running pre-deploy")

	utils.CreateDirectory(n.Cfg.LabDir, 0777)

	// store provided pubkeys
	n.sshPubKeys = params.SSHPubKeys

	// store the certificate-related parameters
	// for cert generation to happen in Post-Deploy phase with mgmt IPs as SANs
	n.cert = params.Cert
	n.topologyName = params.TopologyName

	return n.createSROSFiles()
}

func (n *sros) PostDeploy(ctx context.Context, params *nodes.PostDeployParams) error {
	log.Info("Running postdeploy actions",
		"kind", n.Cfg.Kind,
		"node", n.Cfg.ShortName)

	// generate the certificate
	certificate, err := n.LoadOrGenerateCertificate(n.cert, n.topologyName)
	if err != nil {
		return err
	}

	// set the certificate data
	n.Config().TLSCert = string(certificate.Cert)
	n.Config().TLSKey = string(certificate.Key)

	// Populate /etc/hosts for service discovery on mgmt interface
	if err := n.populateHosts(ctx, params.Nodes); err != nil {
		log.Warnf("Unable to populate hosts for node %q: %v", n.Cfg.ShortName, err)
	}

	// start waiting for initial commit and mgmt server ready
	if err := n.Ready(ctx); err != nil {
		return err
	}

	n.swVersion, err = n.RunningVersion(ctx)
	if err != nil {
		return err
	}

	// return if config file is found in the lab directory.
	// This can be either if the startup-config has been mounted by that path
	// or the config has been previously generated and saved
	if utils.FileExists(filepath.Join(n.Cfg.LabDir, "config", "config.cfg")) {
		return nil
	}

	return nil
}

// Ready returns when the node boot sequence reached the stage when it is ready to accept config commands
// returns an error if not ready by the expiry of the timer readyTimeout.
func (n *sros) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	var err error

	log.Debugf("Waiting for SR-OS node %q to boot...", n.Cfg.ShortName)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for SR-OS node %s to boot: %v", n.Cfg.ShortName, err)
		default:
			//  check if cpm is running
			cmd, _ := exec.NewExecCmdFromString(readyCmd)
			execResult, err := n.RunExec(ctx, cmd)
			if err != nil || (execResult != nil && execResult.GetReturnCode() != 1) {
				logMsg := "CPM status check failed"

				if err != nil {
					logMsg += fmt.Sprintf(" error: %v", err)
				}

				if execResult != nil && execResult.GetReturnCode() != 0 {
					logMsg += fmt.Sprintf(", output: \n%s", execResult)
				}

				log.Debug(logMsg)
				time.Sleep(retryTimer)

				continue
			}

			log.Debugf("Node %s is ready to accept configs", n.Cfg.ShortName)

			return nil
		}
	}
}

// checkKernelVersion emits a warning if the present kernel version is lower than the required one.
func (*sros) checkKernelVersion() error {
	// retrieve running kernel version
	kv, err := utils.GetKernelVersion()
	if err != nil {
		return err
	}

	// do the comparison
	if !kv.GreaterOrEqual(requiredKernelVersion) {
		log.Infof("Nokia SR-OS requires a kernel version greater than %s. Detected kernel version: %s", requiredKernelVersion, kv)
	}
	return nil
}

func (n *sros) CheckDeploymentConditions(ctx context.Context) error {
	// perform the sros specific kernel version check
	err := n.checkKernelVersion()
	if err != nil {
		return err
	}

	return n.DefaultNode.CheckDeploymentConditions(ctx)
}

func (n *sros) createSROSFiles() error {
	log.Debugf("Creating directory structure for SR-OS container: %s", n.Cfg.ShortName)
	var src string
	var err error

	if n.Cfg.License != "" {
		// copy license file to node specific directory in lab
		src = n.Cfg.License
		licPath := filepath.Join(n.Cfg.LabDir, "license.key")
		if err := utils.CopyFile(src, licPath, 0644); err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, licPath, err)
		}
		log.Infof("CopyFile src %s -> dst %s succeeded", src, licPath)
	}

	utils.CreateDirectory(path.Join(n.Cfg.LabDir, "config"), 0777)

	// generate SR-OS topology file
	// err = generateSrosTopology(n.Cfg)
	// if err != nil {
	// 	return err
	// }

	// if n.Cfg.MacAddress != "" {
	// 	srosEnv["NOKIA_SROS_SYSTEM_BASE_MAC"] = n.Cfg.MacAddress
	// }

	// Override NodeType var with existing env
	mac := genMac(n.Cfg)
	if n.Cfg.NodeType != "" {
		srosEnv["NOKIA_SROS_CHASSIS"] = n.Cfg.NodeType
		srosEnv["NOKIA_SROS_SYSTEM_BASE_MAC"] = mac.MAC
	}
	n.Cfg.Env = utils.MergeStringMaps(srosEnv, n.Cfg.Env)
	log.Debugf("Merged env file: %+v for node %q", n.Cfg.Env, n.Cfg.ShortName)
	// log.Infof("HELLO %+v", n.Cfg.Env)
	if _, exists := n.Cfg.Env["NOKIA_SROS_SLOT"]; exists && SlotisInteger(n.Cfg.Env["NOKIA_SROS_SLOT"]) {
		log.Debugf("Skipping config generation for %q because found NOKIA_SROS_SLOT %q is not control plane", n.Cfg.ShortName, n.Cfg.Env["NOKIA_SROS_SLOT"])
	} else {
		err = n.createSROSFilesConfig()
	}
	return err
}

func (n *sros) createSROSFilesConfig() error {
	// generate a startup config file
	// if the node has a `startup-config:` statement, the file specified in that section
	// will be used as a template in GenerateConfig()
	var cfgTemplate string
	var err error
	cfgPath := filepath.Join(n.Cfg.LabDir, "config", "config.cfg")
	if n.Cfg.StartupConfig != "" {
		log.Debugf("Reading startup-config on node %s, file: %s", n.Cfg.ShortName, n.Cfg.StartupConfig)

		c, err := os.ReadFile(n.Cfg.StartupConfig)
		if err != nil {
			return err
		}

		cBuf, err := utils.SubstituteEnvsAndTemplate(bytes.NewReader(c), n.Cfg)
		if err != nil {
			return err
		}

		cfgTemplate = cBuf.String()
	} else {
		// Use default clab config from template
		log.Debugf("Rendering SR-OS default clab config for node=%s...", n.Cfg.ShortName)
		err = n.addDefaultConfig()
		log.Debugf("Rendered default startup Config for node=%s:\n %s", n.Cfg.ShortName, n.startupCliCfg)
		if err := utils.CreateFile(cfgPath, string(n.startupCliCfg)); err != nil {
			return fmt.Errorf("CreateFile file %s failed %v", cfgPath, err)
		}
		log.Infof("CreateFile %s  succeeded", cfgPath)
		if err != nil {
			return err
		}
	}

	if cfgTemplate == "" {
		log.Debugf("configuration template for node %s is empty, skipping startup config file generation", n.Cfg.ShortName)
		return nil
	}

	err = n.GenerateConfig(cfgPath, cfgTemplate)

	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", n.Cfg.ShortName, err)
	}
	return err
}

func (n *sros) srosNodeTypeToLower() {
	nodeType := n.Cfg.NodeType
	n.Cfg.NodeType = strings.ToLower(nodeType)
	log.Debugf("Converted %q NodeType to %q", nodeType, n.Cfg.NodeType)
}

// SlotisInteger checks if the slot string represents a valid integer
func SlotisInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// func generateSrosTopology(cfg *types.NodeConfig) error {
// 	dst := filepath.Join(cfg.LabDir, "topology.yml")
// 	var srosTopology srosTopo

// 	tpl, err := template.ParseFS(topologies, "topology/"+srosTypes[cfg.NodeType])
// 	if err != nil {
// 		return errors.Wrap(err, "failed to parse SR-OS topology template")
// 	}

// 	mac := genMac(cfg)
// 	log.Debugf("MAC: %s, dst: %q", mac, dst)

// 	var buf bytes.Buffer
// 	if err := tpl.Execute(&buf, mac); err != nil {
// 		return err
// 	}

// 	err = yaml.Unmarshal(buf.Bytes(), &srosTopology)
// 	if err != nil {
// 		return err
// 	}

// 	err = utils.CreateFile(dst, buf.String())
// 	if err != nil {
// 		return err
// 	}
// 	log.Debugf("Wrote %s", dst)

// 	// Control Plane Cards
// 	nr_cpms := len(srosTopology.Cpm)
// 	nr_LC := len(srosTopology.Slot)
// 	log.Debugf("SR-OS NodeType %q: TopologyFile: %s, Number of CPMs: %d, LineCards %d", cfg.NodeType, srosTypes[cfg.NodeType], nr_cpms, nr_LC)

// 	if nr_cpms >= 1 && nr_LC > 0 {
// 		if cfg.Extras != nil && cfg.Extras.SROSCard != "" {
// 			cfg.Extras.SROSCard = strings.ToLower(cfg.Extras.SROSCard)
// 			switch cfg.Extras.SROSCard {
// 			case "cpm-a", "a":
// 				log.Debugf("Found match for card %q, topology: %+v", cfg.Extras.SROSCard, srosTopology.Cpm["A"])
// 				_ = assingTopoMapToEnv(srosTopology.Cpm["A"], srosEnv)
// 			case "cpm-b", "b":
// 				log.Debugf("Found match for card %q, topology: %+v", cfg.Extras.SROSCard, srosTopology.Cpm["B"])
// 				_ = assingTopoMapToEnv(srosTopology.Cpm["B"], srosEnv)
// 			// linecard
// 			default:
// 				m := CardRegexp.FindStringSubmatch(cfg.Extras.SROSCard)
// 				if m != nil {
// 					log.Debugf("Found match %q for card %q, topology %+v", m[1], cfg.Extras.SROSCard, srosTopology.Slot[m[1]])
// 					_ = assingTopoMapToEnv(srosTopology.Slot[m[1]], srosEnv)
// 				} else {
// 					return fmt.Errorf("couldn't find a match for `sros-card` %q with regexp `^(?i)(?:iom-|lc-)?([1-9])$`", cfg.Extras.SROSCard)
// 				}
// 			}

// 		} else {
// 			return fmt.Errorf("multi Line Card system node %q (%s) requires option `extras.sros-card` on node definition: CPM: %d, LC: %d", cfg.ShortName, cfg.NodeType, nr_cpms, nr_LC)
// 		}
// 	} else if nr_LC == 0 {
// 		log.Debugf("Single Line Card system %q (%s) detected", cfg.ShortName, cfg.NodeType)
// 		_ = assingTopoMapToEnv(srosTopology.Cpm["A"], srosEnv)
// 	} else {
// 		return fmt.Errorf("oops, multi Line Card system node %q (%s) requires option `extras.sros-card` on node definition: CPM: %d, LC: %d", cfg.ShortName, cfg.NodeType, nr_cpms, nr_LC)
// 	}
// 	return nil
// }

// func assingTopoMapToEnv(topo []map[string]string, target_map map[string]string) error {
// 	for _, item := range topo {
// 		k, kOk := item["name"]
// 		v, vOk := item["value"]
// 		if kOk && vOk {
// 			target_map[k] = v
// 		}
// 	}
// 	return nil
// }

// srosTemplateData top level data struct.
type srosTemplateData struct {
	Name       string
	TLSKey     string
	TLSCert    string
	TLSAnchor  string
	Banner     string
	IFaces     map[string]tplIFace
	SSHPubKeys string
	MgmtMTU    int
	MgmtIPMTU  int
	DNSServers []string
	// EnableGNMIUnixSockServices enables GNMI unix socket services
	// for the node. This is needed for "23.10 <= ver < 24.3" versions
	EnableGNMIUnixSockServices bool
	// EnableCustomPrompt enables custom prompt with added newline
	// before the prompt.
	EnableCustomPrompt bool
	CustomPrompt       string
	// SNMPConfig is a string containing SNMP configuration
	SNMPConfig string
	// GRPCConfig is a string containing GRPC configuration
	GRPCConfig string
	// ACLConfig is a string containing ACL configuration
	ACLConfig string
	// NetconfConfig is a string containing Netconf server configuration
	NetconfConfig string
	// EDAConfig is a string containing EDA configuration
	EDAConfig string
	// OCServerConfig is a string containing OpenConfig server configuration
	OCServerConfig string
	// SystemConfig is a string containing System configuration
	SystemConfig string
	// LoggingConfig is a string containing Logging configuration
	LoggingConfig string
}

// tplIFace template interface struct.
type tplIFace struct {
	Slot       string
	Port       string
	BreakoutNo string
	Mtu        int
}

// addDefaultConfig adds sros default configuration such as tls certs, gnmi/json-rpc, login-banner.
func (n *sros) addDefaultConfig() error {
	b, err := n.banner()
	if err != nil {
		return err
	}

	// tplData holds data used in templating of the default config snippet
	tplData := srosTemplateData{
		Name:          n.Cfg.ShortName,
		TLSKey:        n.Cfg.TLSKey,
		TLSCert:       n.Cfg.TLSCert,
		TLSAnchor:     n.Cfg.TLSAnchor,
		Banner:        b,
		IFaces:        map[string]tplIFace{},
		MgmtMTU:       0,
		MgmtIPMTU:     0,
		DNSServers:    n.Config().DNS.Servers,
		SystemConfig:  systemCfg,
		SNMPConfig:    snmpv2Config,
		GRPCConfig:    grpcConfig,
		NetconfConfig: netconfConfig,
		LoggingConfig: loggingConfig,
	}

	// n.setCustomPrompt(&tplData)

	tplData.MgmtIPMTU = n.Runtime.Mgmt().MTU

	buf := new(bytes.Buffer)
	err = srosCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}
	if buf.Len() == 0 {
		log.Warnf("Buffer empty, Not parsed template data of type %T for node %q", tplData, n.Cfg.ShortName)
	} else {
		log.Debugf("Node %q additional config:\n%s", n.Cfg.ShortName, buf.String())
		n.startupCliCfg = append(n.startupCliCfg, buf.String()...)
	}

	return nil
}

// populateHosts adds container hostnames for other nodes of a lab to SR Linux /etc/hosts file
// to mitigate the fact that srlinux uses non default netns for management and thus
// can't leverage docker DNS service.
func (n *sros) populateHosts(ctx context.Context, nodes map[string]nodes.Node) error {
	hosts, err := n.Runtime.GetHostsPath(ctx, n.Cfg.LongName)
	if err != nil {
		log.Warnf("Unable to locate /etc/hosts file for sros node %v: %v", n.Cfg.ShortName, err)
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

	file, err := os.OpenFile(hosts, os.O_APPEND|os.O_WRONLY, 0666) // skipcq: GSC-G302
	if err != nil {
		log.Warnf("Unable to open /etc/hosts file for sros node %v: %v", n.Cfg.ShortName, err)
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

// // Override method for SR-OS
// func (n *sros) GetMappedInterfaceName(ifName string) (string, error) {
// 	log.Infof("ifName: %s", ifName)
// 	captureGroups, err := utils.GetRegexpCaptureGroups(InterfaceRegexp2, ifName)
// 	if err != nil {
// 		return "", err
// 	}

// 	indexGroups := []string{"card", "connector", "mda", "port", "xiom", "mgmtPort", "fabricPort"}
// 	parsedIndices := make(map[string]int)
// 	foundIndices := make(map[string]bool)
// 	var str string = ""

// 	for _, indexKey := range indexGroups {
// 		if index, found := captureGroups[indexKey]; found && index != "" {
// 			foundIndices[indexKey] = true
// 			parsedIndices[indexKey], err = strconv.Atoi(index)
// 			if err != nil {
// 				return "", fmt.Errorf("%q parsed %s index %q could not be cast to an integer", ifName, indexKey, index)
// 			}
// 		} else {
// 			foundIndices[indexKey] = false
// 		}
// 	}
// 	if foundIndices["card"] && foundIndices["mda"] && foundIndices["port"] {
// 		if foundIndices["connector"] {
// 			if foundIndices["xiom"] {
// 				str = fmt.Sprintf("e%d-x%d-%d-c%d-%d", parsedIndices["card"], parsedIndices["xiom"],
// 					parsedIndices["mda"], parsedIndices["connector"], parsedIndices["port"])
// 			} else {
// 				str = fmt.Sprintf("e%d-%d-c%d-%d", parsedIndices["card"],
// 					parsedIndices["mda"], parsedIndices["connector"], parsedIndices["port"])
// 			}
// 		} else {
// 			str = fmt.Sprintf("e%d-%d-%d", parsedIndices["card"], parsedIndices["mda"], parsedIndices["port"])
// 		}
// 	}
// 	if foundIndices["mgmtPort"] {
// 		str = fmt.Sprintf("eth%d", parsedIndices["mgmtPort"])
// 	}
// 	if foundIndices["fabricPort"] {
// 		str = fmt.Sprintf("fab%d", parsedIndices["fabricPort"])
// 	}
// 	if str != "" {
// 		log.Infof("ifName, %q, MappedIf %q", ifName, str)
// 		return str, nil
// 	} else {
// 		log.Warnf("ifName, %q, MappedIf %q", ifName, str)
// 		return "", fmt.Errorf("%q missing linecard, mda or port index", ifName)
// 	}

// }

// func (n *sros) CalculateInterfaceIndex(ifName string) (int, error) {
// 	captureGroups, err := utils.GetRegexpCaptureGroups(n.InterfaceRegexp, ifName)
// 	if err != nil {
// 		return 0, err
// 	}
// 	indexGroups := []string{"card", "connector", "mda", "port", "xiom", "mgmtPort", "fabricPort"}
// 	parsedIndices := make(map[string]int)
// 	foundIndices := make(map[string]bool)

// 	for _, indexKey := range indexGroups {
// 		if index, found := captureGroups[indexKey]; found && index != "" {
// 			foundIndices[indexKey] = true
// 			parsedIndices[indexKey], err = strconv.Atoi(index)
// 			if err != nil {
// 				return 0, fmt.Errorf("override func CalculateInterfaceIndex: %q parsed index %q could not be cast to an integer", ifName, index)
// 			}
// 		} else {
// 			foundIndices[indexKey] = false
// 		}
// 	}
// 	var calculatedIndex int = -1
// 	if foundIndices["port"] && !foundIndices["connector"] {
// 		calculatedIndex = parsedIndices["port"] - n.InterfaceOffset + n.FirstDataIfIndex
// 	}
// 	if foundIndices["card"] && foundIndices["mda"] && foundIndices["port"] {
// 		if foundIndices["connector"] {
// 			if foundIndices["xiom"] {
// 				calculatedIndex = 1000*parsedIndices["card"] + 100*parsedIndices["xiom"] + 10*parsedIndices["mda"] + parsedIndices["connector"] + parsedIndices["port"] - n.InterfaceOffset + n.FirstDataIfIndex
// 			} else {
// 				calculatedIndex = 100*parsedIndices["card"] + 10 + parsedIndices["mda"] + parsedIndices["connector"] + parsedIndices["port"] - n.InterfaceOffset + n.FirstDataIfIndex
// 			}
// 		} else {
// 			calculatedIndex = 1000*parsedIndices["card"] + 10*parsedIndices["mda"] + parsedIndices["port"] - n.InterfaceOffset + n.FirstDataIfIndex
// 		}
// 	}
// 	if foundIndices["mgmtPort"] {
// 		calculatedIndex = parsedIndices["mgmtPort"] - n.InterfaceOffset + n.FirstDataIfIndex
// 	}
// 	if foundIndices["fabricPort"] {
// 		calculatedIndex = parsedIndices["fabricPort"] + 2 - n.InterfaceOffset + n.FirstDataIfIndex

// 	}
// 	log.Infof("calculatedIndex %d for %s", calculatedIndex, ifName)
// 	if calculatedIndex != -1 {
// 		return calculatedIndex, nil
// 	} else {
// 		return 0, fmt.Errorf("%q does not have extracted interface index with regexp %q, 'port','mgmtPort','fabricPort' capture group missing?", ifName, n.InterfaceRegexp)

// 	}
// }

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (n *sros) CheckInterfaceName() error {
	nm := strings.ToLower(n.Cfg.NetworkMode)

	err := n.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, e := range n.Endpoints {
		// log.Infof("e.GetIfaceName() %q", e.GetIfaceName())
		if !InterfaceRegexp2.MatchString(e.GetIfaceName()) {
			// if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("nokia SR-OS interface name %q doesn't match the required pattern: %s", e.GetIfaceName(), n.InterfaceHelp)
		}

		if e.GetIfaceName() == "eth0" && nm != "none" {
			return fmt.Errorf("eth0 interface name is not allowed for %s node when network mode is not set to none", n.Cfg.ShortName)
		}
	}

	return nil
}

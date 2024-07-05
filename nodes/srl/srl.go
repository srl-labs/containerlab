// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package srl

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	SRLinuxDefaultType = "ixrd2l" // default srl node type

	readyTimeout = time.Minute * 5 // max wait time for node to boot
	retryTimer   = time.Second

	// defaultCfgPath is a path to a file with default config that clab adds on top of the factory config.
	// Default config is a config that adds some basic configuration to the node, such as tls certs, gnmi/json-rpc, login-banner.
	defaultCfgPath = "/tmp/clab-default-config"
	// overlayCfgPath is a path to a file with additional config that clab adds on top of the default config.
	// Partial config provided via startup-config parameter is an overlay config.
	overlayCfgPath = "/tmp/clab-overlay-config"
)

var (

	// additional config that clab adds on top of the factory config.
	//go:embed srl_default_config.go.tpl
	srlConfigCmdsTpl string

	KindNames = []string{"srl", "nokia_srlinux"}
	srlSysctl = map[string]string{
		"net.ipv4.ip_forward":              "0",
		"net.ipv6.conf.all.disable_ipv6":   "0",
		"net.ipv6.conf.all.accept_dad":     "0",
		"net.ipv6.conf.default.accept_dad": "0",
		"net.ipv6.conf.all.autoconf":       "0",
		"net.ipv6.conf.default.autoconf":   "0",
	}
	defaultCredentials = nodes.NewCredentials("admin", "NokiaSrl1!")

	srlTypes = map[string]string{
		"ixsa1":    "7215IXSA1.yml",
		"ixrd1":    "7220IXRD1.yml",
		"ixrd2":    "7220IXRD2.yml",
		"ixrd3":    "7220IXRD3.yml",
		"ixrd2l":   "7220IXRD2L.yml",
		"ixrd3l":   "7220IXRD3L.yml",
		"ixrd4":    "7220IXRD4.yml",
		"ixrd5":    "7220IXRD5.yml",
		"ixrh2":    "7220IXRH2.yml",
		"ixrh3":    "7220IXRH3.yml",
		"ixrh4":    "7220IXRH4.yml",
		"ixr6":     "7250IXR6.yml",
		"ixr6e":    "7250IXR6e.yml",
		"ixr10":    "7250IXR10.yml",
		"ixr10e":   "7250IXR10e.yml",
		"sxr1x44s": "7730SXR-1x-44s.yml",
		"sxr1d32d": "7730SXR-1d-32d.yml",
		"ixrx1b":   "7250IXRX1b.yml",
		"ixrx3b":   "7250IXRX3b.yml",
	}

	srlEnv = map[string]string{"SRLINUX": "1"}

	//go:embed topology/*
	topologies embed.FS

	saveCmd          = `/opt/srlinux/bin/sr_cli -d "tools system configuration save"`
	mgmtServerRdyCmd = `/opt/srlinux/bin/sr_cli -d "info from state system app-management application mgmt_server state | grep running"`
	// readyForConfigCmd checks the output of a file on srlinux which will be populated once the mgmt server is ready to accept config.
	readyForConfigCmd = "cat /etc/opt/srlinux/devices/app_ephemeral.mgmt_server.ready_for_config"

	srlCfgTpl, _ = template.New("clab-srl-default-config").
			Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).
			Parse(srlConfigCmdsTpl)

	requiredKernelVersion = &utils.KernelVersion{
		Major:    4,
		Minor:    10,
		Revision: 0,
	}

	InterfaceRegexp = regexp.MustCompile(`ethernet-(?P<linecard>\d+)/(?P<port>\d+)(?:/(?P<channel>\d+))?`)
	InterfaceHelp   = "ethernet-L/P, ethernet-L/P/C or eL-P, eL-P-C (where L, P, C >= 1)"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(KindNames, func() nodes.Node {
		return new(srl)
	}, defaultCredentials)
}

type srl struct {
	nodes.DefaultNode
	// startup-config passed as a path to a file with CLI instructions will be read into this byte slice
	startupCliCfg []byte

	// Params provided in Pre-Deploy, that srl uses in Post-Deploy phase
	// to generate certificates
	cert         *cert.Cert
	topologyName string
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
	// software version SR Linux node runs
	swVersion *SrlVersion
}

func (n *srl) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)
	// set virtualization requirement
	n.HostRequirements.SSSE3 = true
	n.HostRequirements.MinVCPU = 2
	n.HostRequirements.MinVCPUFailAction = types.FailBehaviourError
	n.HostRequirements.MinAvailMemoryGb = 2
	n.HostRequirements.MinAvailMemoryGbFailAction = types.FailBehaviourLog

	n.Cfg = cfg

	// force cert creation for srlinux nodes as they by make use of tls certificate in the default config
	n.Cfg.Certificate.Issue = utils.Pointer(true)

	for _, o := range opts {
		o(n)
	}

	if n.Cfg.NodeType == "" {
		n.Cfg.NodeType = SRLinuxDefaultType
	}

	if _, found := srlTypes[n.Cfg.NodeType]; !found {
		keys := make([]string, 0, len(srlTypes))
		for key := range srlTypes {
			keys = append(keys, key)
		}
		return fmt.Errorf("wrong node type. '%s' doesn't exist. should be any of %s",
			n.Cfg.NodeType, strings.Join(keys, ", "))
	}

	if n.Cfg.Cmd == "" {
		// set default Cmd if it was not provided by a user
		// the additional touch is needed to support non docker runtimes
		n.Cfg.Cmd = "sudo bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'"
	}

	n.Cfg.Env = utils.MergeStringMaps(srlEnv, n.Cfg.Env)

	// if user was not initialized to a value, use root
	if n.Cfg.User == "" {
		n.Cfg.User = "0:0"
	}
	for k, v := range srlSysctl {
		n.Cfg.Sysctls[k] = v
	}

	if n.Cfg.License != "" {
		// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
		n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(
			filepath.Join(n.Cfg.LabDir, "license.key"), ":/opt/srlinux/etc/license.key:ro"))
	}

	// mount config directory
	cfgPath := filepath.Join(n.Cfg.LabDir, "config")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(cfgPath, ":/etc/opt/srlinux/:rw"))

	// mount srlinux topology
	topoPath := filepath.Join(n.Cfg.LabDir, "topology.yml")
	n.Cfg.Binds = append(n.Cfg.Binds, fmt.Sprint(topoPath, ":/tmp/topology.yml:ro"))

	n.InterfaceRegexp = InterfaceRegexp
	n.InterfaceHelp = InterfaceHelp

	return nil
}

func (s *srl) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)

	// Create appmgr subdir for agent specs and copy files, if needed
	if s.Cfg.Extras != nil && len(s.Cfg.Extras.SRLAgents) != 0 {
		agents := s.Cfg.Extras.SRLAgents

		appmgr := filepath.Join(s.Cfg.LabDir, "config", "appmgr")
		utils.CreateDirectory(appmgr, 0777)

		// process extras -> agents configurations
		for _, fullpath := range agents {
			basename := filepath.Base(fullpath)
			// if it is a url extract filename from url or content-disposition header
			if utils.IsHttpURL(fullpath, false) {
				basename = utils.FilenameForURL(fullpath)
			}
			// enforce yml extension
			if ext := filepath.Ext(basename); !(ext == ".yml" || ext == ".yaml") {
				basename = basename + ".yml"
			}

			dst := filepath.Join(appmgr, basename)
			if err := utils.CopyFile(fullpath, dst, 0644); err != nil {
				return fmt.Errorf("agent copy src %s -> dst %s failed %v", fullpath, dst, err)
			}
		}
	}

	// mount authorized_keys file to enable passwordless login
	authzKeysPath := params.TopoPaths.AuthorizedKeysFilename()
	if utils.FileExists(authzKeysPath) {
		s.Cfg.Binds = append(s.Cfg.Binds,
			fmt.Sprint(authzKeysPath, ":/root/.ssh/authorized_keys:ro"),
			fmt.Sprint(authzKeysPath, ":/home/linuxadmin/.ssh/authorized_keys:ro"),
			fmt.Sprint(authzKeysPath, ":/home/admin/.ssh/authorized_keys:ro"),
		)
	}

	// store provided pubkeys
	s.sshPubKeys = params.SSHPubKeys

	// store the certificate-related parameters
	// for cert generation to happen in Post-Deploy phase with mgmt IPs as SANs
	s.cert = params.Cert
	s.topologyName = params.TopologyName

	return s.createSRLFiles()
}

func (s *srl) PostDeploy(ctx context.Context, params *nodes.PostDeployParams) error {
	log.Infof("Running postdeploy actions for Nokia SR Linux '%s' node", s.Cfg.ShortName)

	// generate the certificate
	certificate, err := s.LoadOrGenerateCertificate(s.cert, s.topologyName)
	if err != nil {
		return err
	}

	// set the certificate data
	s.Config().TLSCert = string(certificate.Cert)
	s.Config().TLSKey = string(certificate.Key)

	// Populate /etc/hosts for service discovery on mgmt interface
	if err := s.populateHosts(ctx, params.Nodes); err != nil {
		log.Warnf("Unable to populate hosts for node %q: %v", s.Cfg.ShortName, err)
	}

	// start waiting for initial commit and mgmt server ready
	if err := s.Ready(ctx); err != nil {
		return err
	}

	s.swVersion, err = s.RunningVersion(ctx)
	if err != nil {
		return err
	}

	// return if config file is found in the lab directory.
	// This can be either if the startup-config has been mounted by that path
	// or the config has been previously generated and saved
	if utils.FileExists(filepath.Join(s.Cfg.LabDir, "config", "config.json")) {
		return nil
	}

	if err := s.addDefaultConfig(ctx); err != nil {
		return err
	}

	if err := s.addOverlayCLIConfig(ctx); err != nil {
		return err
	}

	// once default and overlay config is added, we can commit the config
	if err := s.commitConfig(ctx); err != nil {
		return err
	}

	return s.generateCheckpoint(ctx)
}

func (s *srl) SaveConfig(ctx context.Context) error {
	cmd, _ := exec.NewExecCmdFromString(saveCmd)
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", s.Cfg.ShortName, err)
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("%s errors: %s", s.Cfg.ShortName, execResult.GetStdErrString())
	}

	log.Infof("saved SR Linux configuration from %s node. Output:\n%s", s.Cfg.ShortName, execResult.GetStdOutString())

	return nil
}

// Ready returns when the node boot sequence reached the stage when it is ready to accept config commands
// returns an error if not ready by the expiry of the timer readyTimeout.
func (s *srl) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	var err error

	log.Debugf("Waiting for SR Linux node %q to boot...", s.Cfg.ShortName)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for SR Linux node %s to boot: %v", s.Cfg.ShortName, err)
		default:
			// two commands are checked, first if the mgmt_server is running
			cmd, _ := exec.NewExecCmdFromString(mgmtServerRdyCmd)
			execResult, err := s.RunExec(ctx, cmd)
			if err != nil || (execResult != nil && execResult.GetReturnCode() != 0) {
				logMsg := "mgmt_server status check failed"

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

			if len(execResult.GetStdErrString()) != 0 {
				log.Debugf("error during checking SR Linux boot status: %s", execResult.GetStdErrString())
				time.Sleep(retryTimer)
				continue
			}

			if !strings.Contains(execResult.GetStdOutString(), "running") {
				time.Sleep(retryTimer)
				continue
			}

			// once mgmt server is running, we need to check if it is ready to accept configuration commands
			// this is done with checking readyForConfigCmd
			cmd, _ = exec.NewExecCmdFromString(readyForConfigCmd)
			execResult, err = s.RunExec(ctx, cmd)
			if err != nil {
				log.Debugf("error during readyForConfigCmd execution: %s", err)
				time.Sleep(retryTimer)
				continue
			}

			if len(execResult.GetStdErrString()) != 0 {
				log.Debugf("readyForConfigCmd stderr: %s", string(execResult.GetStdErrString()))
				time.Sleep(retryTimer)
				continue
			}

			if !strings.Contains(execResult.GetStdOutString(), "loaded initial configuration") {
				log.Debugf("Management server readiness files doesn't contain the marker string %s", execResult.GetStdOutString())
				time.Sleep(retryTimer)
				continue
			}

			log.Debugf("Node %s is ready to accept configs", s.Cfg.ShortName)

			return nil
		}
	}
}

// checkKernelVersion emits a warning if the present kernel version is lower than the required one.
func (*srl) checkKernelVersion() error {
	// retrieve running kernel version
	kv, err := utils.GetKernelVersion()
	if err != nil {
		return err
	}

	// do the comparison
	if !kv.GreaterOrEqual(requiredKernelVersion) {
		log.Infof("Nokia SR Linux v23.3.1+ requires a kernel version greater than %s. Detected kernel version: %s", requiredKernelVersion, kv)
	}
	return nil
}

func (s *srl) CheckDeploymentConditions(ctx context.Context) error {
	// perform the srl specific kernel version check
	err := s.checkKernelVersion()
	if err != nil {
		return err
	}

	return s.DefaultNode.CheckDeploymentConditions(ctx)
}

func (s *srl) createSRLFiles() error {
	log.Debugf("Creating directory structure for SRL container: %s", s.Cfg.ShortName)
	var src string

	if s.Cfg.License != "" {
		// copy license file to node specific directory in lab
		src = s.Cfg.License
		licPath := filepath.Join(s.Cfg.LabDir, "license.key")
		if err := utils.CopyFile(src, licPath, 0644); err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, licPath, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, licPath)
	}

	// generate SRL topology file, including base MAC
	err := generateSRLTopologyFile(s.Cfg)
	if err != nil {
		return err
	}

	utils.CreateDirectory(path.Join(s.Cfg.LabDir, "config"), 0777)

	// create repository files (for yum/apt) that
	// are mounted to srl container during the init phase
	err = s.createRepoFiles()
	if err != nil {
		return err
	}

	// generate a startup config file
	// if the node has a `startup-config:` statement, the file specified in that section
	// will be used as a template in GenerateConfig()
	var cfgTemplate string
	cfgPath := filepath.Join(s.Cfg.LabDir, "config", "config.json")
	if s.Cfg.StartupConfig != "" {
		log.Debugf("Reading startup-config %s", s.Cfg.StartupConfig)

		c, err := os.ReadFile(s.Cfg.StartupConfig)
		if err != nil {
			return err
		}

		// Determine if startup-config is a JSON file
		// Get slice of data with optional leading whitespace removed.
		// See RFC 7159, Section 2 for the definition of JSON whitespace.
		x := bytes.TrimLeft(c, " \t\r\n")
		isJSON := len(x) > 0 && x[0] == '{'
		if !isJSON {
			log.Debugf("startup-config passed to %s is in the CLI format. Will apply it in post-deploy stage",
				s.Cfg.ShortName)

			s.startupCliCfg = c

			// no need to generate and mount startup-config passed in a CLI format
			// as we will apply it over the top of a default config in the post deploy stage
			return nil
		}
		cfgTemplate = string(c)
	}

	if cfgTemplate == "" {
		log.Debugf("configuration template for node %s is empty, skipping startup config file generation", s.Cfg.ShortName)

		return nil
	}

	err = s.GenerateConfig(cfgPath, cfgTemplate)
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", s.Cfg.ShortName, err)
	}

	return err
}

func generateSRLTopologyFile(cfg *types.NodeConfig) error {
	dst := filepath.Join(cfg.LabDir, "topology.yml")

	tpl, err := template.ParseFS(topologies, "topology/"+srlTypes[cfg.NodeType])
	if err != nil {
		return errors.Wrap(err, "failed to get srl topology file")
	}

	mac := genMac(cfg)

	log.Debug(mac, dst)

	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	if err := tpl.Execute(f, mac); err != nil {
		return err
	}

	return f.Close()
}

// srlTemplateData top level data struct.
type srlTemplateData struct {
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
}

// tplIFace template interface struct.
type tplIFace struct {
	Slot       string
	Port       string
	BreakoutNo string
	Mtu        int
}

// addDefaultConfig adds srl default configuration such as tls certs, gnmi/json-rpc, login-banner.
func (n *srl) addDefaultConfig(ctx context.Context) error {
	b, err := n.banner()
	if err != nil {
		return err
	}

	// tplData holds data used in templating of the default config snippet
	tplData := srlTemplateData{
		TLSKey:     n.Cfg.TLSKey,
		TLSCert:    n.Cfg.TLSCert,
		TLSAnchor:  n.Cfg.TLSAnchor,
		Banner:     b,
		IFaces:     map[string]tplIFace{},
		MgmtMTU:    0,
		MgmtIPMTU:  0,
		DNSServers: n.Config().DNS.Servers,
		SNMPConfig: snmpv2Config,
		GRPCConfig: grpcConfig,
	}

	n.setVersionSpecificParams(&tplData)

	n.setCustomPrompt(&tplData)

	// set MgmtMTU to the MTU value of the runtime management network
	// so that the two MTUs match.
	tplData.MgmtIPMTU = n.Runtime.Mgmt().MTU

	// prepare the endpoints
	for _, e := range n.Endpoints {
		ifName := e.GetIfaceName()
		if ifName == "mgmt0" {
			// if the endpoint has a custom MTU set, use it in the template logic
			// otherwise we don't set the mtu as srlinux will use the default max value 9232
			if m := e.GetLink().GetMTU(); m != links.DefaultLinkMTU {
				tplData.MgmtMTU = m
				// MgmtMTU seems to be only set when we use macvlan interface
				// with network-mode: none. For this super narrow use case
				// we setup mgmt port mtu to match the mtu of the macvlan parnet interface
				// but then we need to make sure that IP MTU is smaller by 14B
				tplData.MgmtIPMTU = m - 14
			}
			// the rest is just for traffic carrying interfaces
			continue
		}
		// split the interface identifier into their parts
		ifNameParts := strings.SplitN(strings.TrimLeft(ifName, "e"), "-", 3)

		// create a template interface struct
		iface := tplIFace{
			Slot: ifNameParts[0],
			Port: ifNameParts[1],
		}
		// if it is a breakout port add the breakout identifier
		if len(ifNameParts) == 3 {
			iface.BreakoutNo = ifNameParts[2]
		}

		// if the endpoint has a custom MTU set, use it in the template logic
		// otherwise we don't set the mtu as srlinux will use the default max value 9232
		if m := e.GetLink().GetMTU(); m != links.DefaultLinkMTU {
			iface.Mtu = m
		}

		// add the template interface definition to the template data
		tplData.IFaces[ifName] = iface
	}

	buf := new(bytes.Buffer)
	err = srlCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}

	log.Debugf("Node %q additional config:\n%s", n.Cfg.ShortName, buf.String())

	execCmd := exec.NewExecCmdFromSlice([]string{
		"bash", "-c",
		fmt.Sprintf("echo '%s' > %s", buf.String(), defaultCfgPath),
	})
	_, err = n.RunExec(ctx, execCmd)
	if err != nil {
		return err
	}

	// su to admin user to apply the default config
	// to make sure that the 'environment save' command will create
	// files with correct permissions
	cmd := exec.NewExecCmdFromSlice([]string{
		"bash", "-c",
		fmt.Sprintf("su -s /bin/bash admin -c '/opt/srlinux/bin/sr_cli -ed < %s'", defaultCfgPath),
	})

	execResult, err := n.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", n.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return nil
}

// addOverlayCLIConfig adds CLI formatted config that is read out of a file provided via startup-config directive.
func (s *srl) addOverlayCLIConfig(ctx context.Context) error {
	if len(s.startupCliCfg) == 0 {
		log.Debugf("node %q: startup-config empty, committing existing candidate", s.Config().ShortName)

		return nil
	}

	cfgStr := string(s.startupCliCfg)

	log.Debugf("Node %q additional config from startup-config file %s:\n%s", s.Cfg.ShortName, s.Cfg.StartupConfig, cfgStr)

	cmd := exec.NewExecCmdFromSlice([]string{
		"bash", "-c",
		fmt.Sprintf("echo '%s' > %s", cfgStr, overlayCfgPath),
	})
	_, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	cmd = exec.NewExecCmdFromSlice([]string{
		"bash", "-c",
		fmt.Sprintf("su -s /bin/bash admin -c '/opt/srlinux/bin/sr_cli -ed < %s'", overlayCfgPath),
	})
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) != 0 {
		return fmt.Errorf("%w:%s", nodes.ErrCommandExecError, execResult.GetStdErrString())
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", s.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return nil
}

// commitConfig commits and saves default+overlay config to the startup-config file.
func (s *srl) commitConfig(ctx context.Context) error {
	log.Debugf("Node %q: commiting configuration", s.Cfg.ShortName)

	cmd, err := exec.NewExecCmdFromString(`bash -c "/opt/srlinux/bin/sr_cli -ed commit save"`)
	if err != nil {
		return err
	}
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) != 0 {
		return fmt.Errorf("%w:%s", nodes.ErrCommandExecError, execResult.GetStdErrString())
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", s.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return nil
}

func (s *srl) generateCheckpoint(ctx context.Context) error {
	cmd, err := exec.NewExecCmdFromString(`bash -c '/opt/srlinux/bin/sr_cli /tools system configuration generate-checkpoint name clab-initial comment \"set by containerlab\"'`)
	if err != nil {
		return err
	}

	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) != 0 {
		return fmt.Errorf("%w:%s", nodes.ErrCommandExecError, execResult.GetStdErrString())
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", s.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return nil
}

// populateHosts adds container hostnames for other nodes of a lab to SR Linux /etc/hosts file
// to mitigate the fact that srlinux uses non default netns for management and thus
// can't leverage docker DNS service.
func (s *srl) populateHosts(ctx context.Context, nodes map[string]nodes.Node) error {
	hosts, err := s.Runtime.GetHostsPath(ctx, s.Cfg.LongName)
	if err != nil {
		log.Warnf("Unable to locate /etc/hosts file for srl node %v: %v", s.Cfg.ShortName, err)
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
		log.Warnf("Unable to open /etc/hosts file for srl node %v: %v", s.Cfg.ShortName, err)
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

func (s *srl) GetMappedInterfaceName(ifName string) (string, error) {
	captureGroups, err := utils.GetRegexpCaptureGroups(s.InterfaceRegexp, ifName)

	if err != nil {
		return "", err
	}

	indexGroups := []string{"linecard", "port", "channel"}
	parsedIndices := make(map[string]int)
	foundIndices := make(map[string]bool)

	for _, indexKey := range indexGroups {
		if index, found := captureGroups[indexKey]; found && index != "" {
			foundIndices[indexKey] = true
			parsedIndices[indexKey], err = strconv.Atoi(index)
			if err != nil {
				return "", fmt.Errorf("%q parsed %s index %q could not be cast to an integer", ifName, indexKey, index)
			}
			if !(parsedIndices[indexKey] >= 1) {
				return "", fmt.Errorf("%q parsed %q index %q does not match requirement >= 1", ifName, indexKey, index)
			}
		} else {
			foundIndices[indexKey] = false
		}
	}

	if foundIndices["linecard"] && foundIndices["port"] {
		if foundIndices["channel"] {
			return fmt.Sprintf("e%d-%d-%d", parsedIndices["linecard"], parsedIndices["port"], parsedIndices["channel"]), nil
		} else {
			return fmt.Sprintf("e%d-%d", parsedIndices["linecard"], parsedIndices["port"]), nil
		}
	} else {
		return "", fmt.Errorf("%q missing linecard or port index", ifName)
	}
}

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (s *srl) CheckInterfaceName() error {
	// allow ethernetX-X-X, eX-X-X and mgmt0 interface names
	ifRe := regexp.MustCompile(`(:?e|ethernet)\d+-\d+(-\d+)?|mgmt0`)
	nm := strings.ToLower(s.Cfg.NetworkMode)

	err := s.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, e := range s.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("nokia sr linux interface name %q doesn't match the required pattern: %s", e.GetIfaceName(), s.InterfaceHelp)
		}

		if e.GetIfaceName() == "mgmt0" && nm != "none" {
			return fmt.Errorf("mgmt0 interface name is not allowed for %s node when network mode is not set to none", s.Cfg.ShortName)
		}
	}

	return nil
}

// createRepoFiles creates apt/ym repository files
// to enable srl nodes to install ndk apps.
func (s *srl) createRepoFiles() error {
	yumRepo := `[srlinux]
name=SR Linux NDK apps
baseurl=https://srlinux.fury.site/yum/
enabled=1
gpgcheck=0`

	aptRepo := `deb [trusted=yes] https://srlinux.fury.site/apt/ /`

	yumPath := s.Cfg.LabDir + "/yum.repo"
	err := utils.CreateFile(yumPath, yumRepo)
	if err != nil {
		return err
	}

	aptPath := s.Cfg.LabDir + "/apt.list"
	err = utils.CreateFile(aptPath, aptRepo)
	if err != nil {
		return err
	}

	// mount srlinux repository files
	s.Cfg.Binds = append(s.Cfg.Binds, yumPath+":/etc/yum.repos.d/srlinux.repo:ro")
	s.Cfg.Binds = append(s.Cfg.Binds, aptPath+":/etc/apt/sources.list.d/srlinux.list:ro")

	return nil
}

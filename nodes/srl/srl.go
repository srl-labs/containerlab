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
	"strings"
	"text/template"
	"time"

	"github.com/google/shlex"
	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	srlDefaultType = "ixrd2"

	readyTimeout = time.Minute * 2 // max wait time for node to boot
	retryTimer   = time.Second
	// additional config that clab adds on top of the factory config.
	srlConfigCmdsTpl = `set / system tls server-profile clab-profile
set / system tls server-profile clab-profile key "{{ .TLSKey }}"
set / system tls server-profile clab-profile certificate "{{ .TLSCert }}"
{{- if .TLSAnchor }}
set / system tls server-profile clab-profile authenticate-client true
set / system tls server-profile clab-profile trust-anchor "{{ .TLSAnchor }}"
{{- else }}
set / system tls server-profile clab-profile authenticate-client false
{{- end }}
set / system gnmi-server admin-state enable network-instance mgmt admin-state enable tls-profile clab-profile
set / system gnmi-server unix-socket admin-state enable
set / system json-rpc-server admin-state enable network-instance mgmt http admin-state enable
set / system json-rpc-server admin-state enable network-instance mgmt https admin-state enable tls-profile clab-profile
set / system lldp admin-state enable
set / system aaa authentication idle-timeout 7200
{{/* enabling interfaces referenced as endpoints for a node (both e1-2 and e1-3-1 notations) */}}
{{- range $ep := .Endpoints }}
{{- $parts := ($ep.EndpointName | strings.ReplaceAll "e" "" | strings.Split "-") -}}
set / interface ethernet-{{index $parts 0}}/{{index $parts 1}} admin-state enable
  {{- if eq (len $parts) 3 }}
set / interface ethernet-{{index $parts 0}}/{{index $parts 1}} breakout-mode num-channels 4 channel-speed 25G
set / interface ethernet-{{index $parts 0}}/{{index $parts 1}}/{{index $parts 2}} admin-state enable
  {{- end }}
{{ end -}}
set / system banner login-banner "{{ .Banner }}"
commit save`
)

var (
	kindnames = []string{"srl", "nokia_srlinux"}
	srlSysctl = map[string]string{
		"net.ipv4.ip_forward":              "0",
		"net.ipv6.conf.all.disable_ipv6":   "0",
		"net.ipv6.conf.all.accept_dad":     "0",
		"net.ipv6.conf.default.accept_dad": "0",
		"net.ipv6.conf.all.autoconf":       "0",
		"net.ipv6.conf.default.autoconf":   "0",
	}

	srlTypes = map[string]string{
		"ixrd1":  "7220IXRD1.yml",
		"ixrd2":  "7220IXRD2.yml",
		"ixrd3":  "7220IXRD3.yml",
		"ixrd2l": "7220IXRD2L.yml",
		"ixrd3l": "7220IXRD3L.yml",
		"ixrd5":  "7220IXRD5.yml",
		"ixrh2":  "7220IXRH2.yml",
		"ixrh3":  "7220IXRH3.yml",
		"ixr6":   "7250IXR6.yml",
		"ixr6e":  "7250IXR6e.yml",
		"ixr10":  "7250IXR10.yml",
		"ixr10e": "7250IXR10e.yml",
	}

	srlEnv = map[string]string{"SRLINUX": "1"}

	//go:embed topology/*
	topologies embed.FS

	saveCmd             = []string{"sr_cli", "-d", "tools", "system", "configuration", "save"}
	mgmtServerRdyCmd, _ = shlex.Split("sr_cli -d info from state system app-management application mgmt_server state | grep running")
	// readyForConfigCmd checks the output of a file on srlinux which will be populated once the mgmt server is ready to accept config.
	readyForConfigCmd, _ = shlex.Split("cat /etc/opt/srlinux/devices/app_ephemeral.mgmt_server.ready_for_config")

	srlCfgTpl, _ = template.New("srl-tls-profile").
			Funcs(gomplate.CreateFuncs(context.Background(), new(data.Data))).
			Parse(srlConfigCmdsTpl)
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(srl)
	})
	err := nodes.SetDefaultCredentials(kindnames, "admin", "admin")
	if err != nil {
		log.Error(err)
	}
}

type srl struct {
	nodes.DefaultNode
	// startup-config passed as a path to a file with CLI instructions will be read into this byte slice
	startupCliCfg []byte
}

func (s *srl) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)
	// set virtualization requirement
	s.HostRequirements.SSSE3 = true

	s.Cfg = cfg
	// TODO: this is just a QUICKFIX. clab/config.go needs to be fixed
	// to not rely on certain kind names
	s.Cfg.Kind = nodes.NodeKindSRL
	for _, o := range opts {
		o(s)
	}

	if s.Cfg.NodeType == "" {
		s.Cfg.NodeType = srlDefaultType
	}

	if _, found := srlTypes[s.Cfg.NodeType]; !found {
		keys := make([]string, 0, len(srlTypes))
		for key := range srlTypes {
			keys = append(keys, key)
		}
		return fmt.Errorf("wrong node type. '%s' doesn't exist. should be any of %s",
			s.Cfg.NodeType, strings.Join(keys, ", "))
	}

	if s.Cfg.Cmd == "" {
		// set default Cmd if it was not provided by a user
		// the additional touch is needed to support non docker runtimes
		s.Cfg.Cmd = "sudo bash -c 'touch /.dockerenv && /opt/srlinux/bin/sr_linux'"
	}

	s.Cfg.Env = utils.MergeStringMaps(srlEnv, s.Cfg.Env)

	// if user was not initialized to a value, use root
	if s.Cfg.User == "" {
		s.Cfg.User = "0:0"
	}
	for k, v := range srlSysctl {
		s.Cfg.Sysctls[k] = v
	}

	if s.Cfg.License != "" {
		// we mount a fixed path node.Labdir/license.key as the license referenced in topo file will be copied to that path
		s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(
			filepath.Join(s.Cfg.LabDir, "license.key"), ":/opt/srlinux/etc/license.key:ro"))
	}

	// mount config directory
	cfgPath := filepath.Join(s.Cfg.LabDir, "config")
	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(cfgPath, ":/etc/opt/srlinux/:rw"))

	// mount srlinux topology
	topoPath := filepath.Join(s.Cfg.LabDir, "topology.yml")
	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(topoPath, ":/tmp/topology.yml:ro"))

	return nil
}

func (s *srl) PreDeploy(_ context.Context, configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	// retrieve node certificates
	nodeCerts, err := cert.RetrieveNodeCertData(s.Cfg, labCADir)
	// if not available on disk, create cert in next step
	if err != nil {
		// create CERT
		certTpl, err := template.New("node-cert").Parse(cert.NodeCSRTempl)
		if err != nil {
			log.Errorf("failed to parse Node CSR Template: %v", err)
		}
		log.Debugf("creating node certificate for %s", s.Cfg.SANs)

		certInput := cert.CertInput{
			Name:     s.Cfg.ShortName,
			LongName: s.Cfg.LongName,
			Fqdn:     s.Cfg.Fqdn,
			SANs:     s.Cfg.SANs,
			Prefix:   configName,
		}
		nodeCerts, err = cert.GenerateCert(
			path.Join(labCARoot, "root-ca.pem"),
			path.Join(labCARoot, "root-ca-key.pem"),
			certTpl,
			certInput,
			path.Join(labCADir, certInput.Name),
		)
		if err != nil {
			log.Errorf("failed to generate certificates for node %s: %v", s.Cfg.ShortName, err)
		}
		log.Debugf("%s CSR: %s", s.Cfg.ShortName, string(nodeCerts.Csr))
		log.Debugf("%s Cert: %s", s.Cfg.ShortName, string(nodeCerts.Cert))
		log.Debugf("%s Key: %s", s.Cfg.ShortName, string(nodeCerts.Key))
	}
	s.Cfg.TLSCert = string(nodeCerts.Cert)
	s.Cfg.TLSKey = string(nodeCerts.Key)

	// Create appmgr subdir for agent specs and copy files, if needed
	if s.Cfg.Extras != nil && len(s.Cfg.Extras.SRLAgents) != 0 {
		agents := s.Cfg.Extras.SRLAgents

		appmgr := filepath.Join(s.Cfg.LabDir, "config", "appmgr")

		utils.CreateDirectory(appmgr, 0777)

		for _, fullpath := range agents {
			basename := filepath.Base(fullpath)
			dst := filepath.Join(appmgr, basename)
			if err := utils.CopyFile(fullpath, dst, 0644); err != nil {
				return fmt.Errorf("agent copy src %s -> dst %s failed %v", fullpath, dst, err)
			}
		}
	}

	// mount authorized_keys file to enable passwordless login
	authzKeysPath := filepath.Join(filepath.Dir(s.Cfg.LabDir), "authorized_keys")
	if utils.FileExists(authzKeysPath) {
		s.Cfg.Binds = append(s.Cfg.Binds,
			fmt.Sprint(authzKeysPath, ":/root/.ssh/authorized_keys:ro"),
			fmt.Sprint(authzKeysPath, ":/home/linuxadmin/.ssh/authorized_keys:ro"),
			fmt.Sprint(authzKeysPath, ":/home/admin/.ssh/authorized_keys:ro"),
		)
	}

	return s.createSRLFiles()
}

func (s *srl) PostDeploy(ctx context.Context, _ map[string]nodes.Node, nodesRuntimeInfo []types.GenericContainer) error {
	log.Infof("Running postdeploy actions for Nokia SR Linux '%s' node", s.Cfg.ShortName)

	// Populate /etc/hosts for service discovery on mgmt interface
	if err := s.populateHosts(ctx, nodesRuntimeInfo); err != nil {
		log.Warnf("Unable to populate hosts for node %q: %v", s.Cfg.ShortName, err)
	}

	// start waiting for initial commit and mgmt server ready
	if err := s.Ready(ctx); err != nil {
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

	return s.addOverlayCLIConfig(ctx)
}

func (s *srl) SaveConfig(ctx context.Context) error {
	exec := types.NewExecSlice(saveCmd)
	execResult, err := s.RunExecType(ctx, exec)

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
			exec := types.NewExecSlice(mgmtServerRdyCmd)
			execResult, err := s.RunExecType(ctx, exec)
			if err != nil {
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
			exec = types.NewExecSlice(readyForConfigCmd)
			execResult, err = s.RunExecType(ctx, exec)
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

func (s *srl) createSRLFiles() error {
	log.Debugf("Creating directory structure for SRL container: %s", s.Cfg.ShortName)
	var src string
	var dst string

	if s.Cfg.License != "" {
		// copy license file to node specific directory in lab
		src = s.Cfg.License
		dst = filepath.Join(s.Cfg.LabDir, "license.key")
		if err := utils.CopyFile(src, dst, 0644); err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}

	// generate SRL topology file, including base MAC
	err := generateSRLTopologyFile(s.Cfg)
	if err != nil {
		return err
	}

	utils.CreateDirectory(path.Join(s.Cfg.LabDir, "config"), 0777)

	// generate a startup config file
	// if the node has a `startup-config:` statement, the file specified in that section
	// will be used as a template in GenerateConfig()
	if s.Cfg.StartupConfig != "" {
		dst = filepath.Join(s.Cfg.LabDir, "config", "config.json")

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

		cfgTemplate := string(c)

		err = s.GenerateConfig(dst, cfgTemplate)
		if err != nil {
			log.Errorf("node=%s, failed to generate config: %v", s.Cfg.ShortName, err)
		}
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

// addDefaultConfig adds srl default configuration such as tls certs, gnmi/json-rpc, login-banner.
func (s *srl) addDefaultConfig(ctx context.Context) error {
	b, err := s.banner(ctx)
	if err != nil {
		return err
	}

	// struct that holds data used in templating of the default config snippet
	tplData := struct {
		*types.NodeConfig
		Banner string
	}{
		s.Cfg,
		b,
	}

	buf := new(bytes.Buffer)
	err = srlCfgTpl.Execute(buf, tplData)
	if err != nil {
		return err
	}

	log.Debugf("Node %q additional config:\n%s", s.Cfg.ShortName, buf.String())

	exec := types.NewExecSlice([]string{"bash", "-c", fmt.Sprintf("echo '%s' > /tmp/clab-config", buf.String())})
	_, err = s.RunExecType(ctx, exec)
	if err != nil {
		return err
	}

	exec = types.NewExecSlice([]string{"bash", "-c", "sr_cli -ed < tmp/clab-config"})
	execResult, err := s.RunExecType(ctx, exec)
	if err != nil {
		return err
	}

	log.Debugf("node %s. stdout: %s, stderr: %s", s.Cfg.ShortName, execResult.GetStdOutString(), execResult.GetStdErrString())

	return nil
}

// addOverlayCLIConfig adds CLI formatted config that is read out of a file provided via startup-config directive.
func (s *srl) addOverlayCLIConfig(ctx context.Context) error {
	cfgStr := string(s.startupCliCfg)

	log.Debugf("Node %q additional config from startup-config file %s:\n%s", s.Cfg.ShortName, s.Cfg.StartupConfig, cfgStr)

	exec := types.NewExecSlice([]string{"bash", "-c", fmt.Sprintf("echo '%s' > /tmp/clab-config", cfgStr)})
	_, err := s.RunExecType(ctx, exec)
	if err != nil {
		return err
	}

	exec = types.NewExecSlice([]string{"bash", "-c", "sr_cli -ed --post 'commit save' < tmp/clab-config"})
	execResult, err := s.RunExecType(ctx, exec)
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
func (s *srl) populateHosts(ctx context.Context, nodesRuntimeInfos []types.GenericContainer) error {
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

	for _, nodeRuntimeInfo := range nodesRuntimeInfos {
		if v4 := nodeRuntimeInfo.NetworkSettings.IPv4addr; v4 != "" {
			fmt.Fprintf(&entriesv4, "%s\t%s\n", v4, nodeRuntimeInfo.Names[0])
		}
		if v6 := nodeRuntimeInfo.NetworkSettings.IPv6addr; v6 != "" {
			fmt.Fprintf(&entriesv6, "%s\t%s\n", v6, nodeRuntimeInfo.Names[0])
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

func (s *srl) CheckInterfaceNamingConvention() error {
	srlIfRe := regexp.MustCompile(`e\d+-\d+(-\d+)?`)
	for _, e := range s.Config().Endpoints {
		if !srlIfRe.MatchString(e.EndpointName) {
			return fmt.Errorf("nokia sr linux endpoint %q doesn't match required pattern. SR Linux endpoints should be named as e1-1 or e1-1-1", e.EndpointName)
		}
	}
	return nil
}

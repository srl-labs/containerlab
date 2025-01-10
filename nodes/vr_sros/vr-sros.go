// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_sros

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/options"
	scraplilogging "github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
)

var (
	kindNames          = []string{"nokia_sros", "vr-sros", "vr-nokia_sros"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")

	InterfaceRegexp = regexp.MustCompile(`1/1/(?P<port>\d+)`)
	InterfaceOffset = 1
	InterfaceHelp   = "1/1/X (where X >= 1) or ethX (where X >= 1)"
)

const (
	generateable     = true
	generateIfFormat = "1/1/%d"

	vrsrosDefaultType   = "sr-1"
	scrapliPlatformName = "nokia_sros"
	configDirName       = "tftpboot"
	startupCfgFName     = "config.txt"
	licenseFName        = "license.txt"
)

// SROSTemplateData holds ssh keys for template generation.
type SROSTemplateData struct {
	SSHPubKeysRSA   []string
	SSHPubKeysECDSA []string
}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes)
	r.Register(kindNames, func() nodes.Node {
		return new(vrSROS)
	}, nrea)
}

type vrSROS struct {
	nodes.VRNode
	// SSH public keys extracted from the clab host
	sshPubKeys []ssh.PublicKey
}

func (s *vrSROS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.VRNode = *nodes.NewVRNode(s)
	// set virtualization requirement
	s.HostRequirements.VirtRequired = true
	s.LicensePolicy = types.LicensePolicyWarn
	// SR OS requires unbound pubkey authentication mode until this is
	// gets fixed in later SR OS release.
	s.SSHConfig.PubkeyAuthentication = types.PubkeyAuthValueUnbound

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// vr-sros type sets the vrnetlab/sros variant (https://github.com/hellt/vrnetlab/sros)
	if s.Cfg.NodeType == "" {
		s.Cfg.NodeType = vrsrosDefaultType
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": s.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.Mgmt.IPv6Subnet,
	}
	s.Cfg.Env = utils.MergeStringMaps(defEnv, s.Cfg.Env)

	// mount tftpboot dir
	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(path.Join(s.Cfg.LabDir, "tftpboot"), ":/tftpboot"))
	if s.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.Cfg.Binds = append(s.Cfg.Binds, "/dev:/dev")
	}

	s.Cfg.Cmd = fmt.Sprintf("--trace --connection-mode %s --hostname %s --variant %q", s.Cfg.Env["CONNECTION_MODE"],
		s.Cfg.ShortName,
		s.Cfg.NodeType,
	)

	s.InterfaceRegexp = InterfaceRegexp
	s.InterfaceOffset = InterfaceOffset
	s.InterfaceHelp = InterfaceHelp

	return nil
}

func (s *vrSROS) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}

	// store public keys extracted from clab host
	s.sshPubKeys = params.SSHPubKeys

	return createVrSROSFiles(s)
}

func (s *vrSROS) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	// b holds the configuration to be applied to the node
	b := &bytes.Buffer{}

	// apply partial configs if partial config is used and existing node config does not exist
	if isPartialConfigFile(s.Cfg.StartupConfig) && !nodeConfigExists(s.Cfg.LabDir) {
		log.Infof("%s: adding config from %s", s.Cfg.LongName, s.Cfg.StartupConfig)

		r, err := os.Open(s.Cfg.StartupConfig)
		if err != nil {
			return err
		}

		defer r.Close() // skipcq: GO-S2307

		_, err = io.Copy(b, r)
		if err != nil {
			return err
		}
	}

	// skip ssh key configuration if CLAB_SKIP_SROS_SSH_KEY_CONFIG env var is set
	// which is needed for SR OS nodes running in classic CLI mode, because our key
	// injection mechanism assumes MD-CLI mode.
	_, skipSSHKeyCfg := os.LookupEnv("CLAB_SKIP_SROS_SSH_KEY_CONFIG")

	if len(s.sshPubKeys) > 0 && !skipSSHKeyCfg {
		log.Infof("%s: adding public keys configuration", s.Cfg.LongName)

		sshConf, err := s.generateSSHPublicKeysConfig()
		if err != nil {
			return err
		}

		_, err = io.Copy(b, sshConf)
		if err != nil {
			return err
		}
	}

	// apply the aggregated config snippets
	if b.Len() > 0 {
		err := s.applyPartialConfig(ctx, s.Cfg.MgmtIPv4Address, scrapliPlatformName,
			defaultCredentials.GetUsername(), defaultCredentials.GetPassword(),
			b,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *vrSROS) SaveConfig(_ context.Context) error {
	err := netconf.SaveConfig(s.Cfg.LongName,
		defaultCredentials.GetUsername(),
		defaultCredentials.GetPassword(),
		scrapliPlatformName,
	)
	if err != nil {
		return err
	}

	log.Infof("saved %s running configuration to startup configuration file\n", s.Cfg.ShortName)
	return nil
}

func createVrSROSFiles(node nodes.Node) error {
	nodeCfg := node.Config()

	// use default startup config load function if config in full form is provided
	if !isPartialConfigFile(nodeCfg.StartupConfig) {
		// do not create new config file if there's existing config file
		if nodeConfigExists(nodeCfg.LabDir) {
			log.Infof("Using existing config file (%s) instead of applying a new one",
				filepath.Join(nodeCfg.LabDir, configDirName, startupCfgFName))
		} else {
			nodes.LoadStartupConfigFileVr(node, configDirName, startupCfgFName)
		}
	}

	if nodeCfg.License != "" {
		// copy license file to node specific lab directory
		src := nodeCfg.License
		dst := filepath.Join(nodeCfg.LabDir, configDirName, licenseFName)
		if err := utils.CopyFile(src, dst, 0644); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}

	return nil
}

// isPartialConfigFile returns true if the config file name contains .partial substring.
func isPartialConfigFile(c string) bool {
	return strings.Contains(strings.ToUpper(c), ".PARTIAL")
}

// nodeConfigExists returns true if a file at <labdir>/<node>/tftpboot/config.txt exists.
func nodeConfigExists(labDir string) bool {
	_, err := os.Stat(filepath.Join(labDir, configDirName, startupCfgFName))
	return err == nil
}

// isHealthy checks if the "/health" file created by vrnetlab exists and contains "0 running".
func (s *vrSROS) isHealthy(ctx context.Context) bool {
	ex := exec.NewExecCmdFromSlice([]string{"grep", "0 running", "/health"})

	res, err := s.RunExec(ctx, ex)
	if err != nil {
		return false
	}

	log.Debugf("Node %q health status: %v", s.Cfg.ShortName, res.ReturnCode == 0)

	return res.ReturnCode == 0
}

// applyPartialConfig applies partial configuration to the SR OS.
func (s *vrSROS) applyPartialConfig(ctx context.Context, addr, platformName,
	username, password string, config io.Reader,
) error { // skipcq: GO-R1005
	var err error
	var d *network.Driver

	configContent, err := io.ReadAll(config)
	if err != nil {
		return err
	}

	configContentStr := string(configContent)

	// check file contains content, otherwise exit early
	if strings.TrimSpace(configContentStr) == "" {
		return nil
	}

	log.Infof("Waiting for %[1]s to be ready. This may take a while. Monitor boot log with `sudo docker logs -f %[1]s`", s.Cfg.LongName)
	for loop := true; loop; {
		if !s.isHealthy(ctx) {
			time.Sleep(5 * time.Second) // cool-off period
			log.Debugf("Waiting for %s to become healthy", s.Cfg.ShortName)
			continue
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: timed out waiting to accept configs", addr)
		default:
			li, err := scraplilogging.NewInstance(
				scraplilogging.WithLevel("debug"),
				scraplilogging.WithLogger(log.Debugln))
			if err != nil {
				return err
			}

			opts := []util.Option{
				options.WithAuthNoStrictKey(),
				options.WithAuthUsername(username),
				options.WithAuthPassword(password),
				options.WithTransportType(transport.StandardTransport),
				options.WithTimeoutOps(5 * time.Second),
				options.WithLogger(li),
			}

			p, err := platform.NewPlatform(platformName, addr, opts...)
			if err != nil {
				return fmt.Errorf("%s: failed to create platform: %+v", addr, err)
			}

			d, err = p.GetNetworkDriver()
			if err != nil {
				return fmt.Errorf("%s: could not create the driver: %+v", addr, err)
			}

			err = d.Open()
			if err == nil {
				// driver successfully opened, exit the loop
				loop = false
			} else {
				log.Debugf("%s: not yet ready - %v", addr, err)
				time.Sleep(5 * time.Second) // cool-off period
			}
		}
	}

	// Normalize character sequences to avoid interaction issues with CLI
	replacer := strings.NewReplacer(
		"\r\n", "\n", // replace EOL CRLF with LF
		"\t", "    ", // replace tabs with 4 spaces
	)
	configContentStr = replacer.Replace(configContentStr)

	// converting string to newline delimited string slice
	cfgs := strings.Split(configContentStr, "\n")

	// config snippets should not have commit command, so we need to commit manually
	// and quit from the config mode
	cfgs = append(cfgs, "commit", "/admin save", "/exit all", "quit-config")

	mr, err := d.SendConfigs(cfgs)
	if err != nil || (mr != nil && mr.Failed != nil) {
		if mr != nil {
			return fmt.Errorf("failed to apply config; error: %+v %+v", err, mr.Failed)
		} else {
			return fmt.Errorf("failed to apply config; error: %+v", err)
		}
	}

	return nil
}

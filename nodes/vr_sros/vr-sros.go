// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_sros

import (
	"context"
	"fmt"
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
)

var (
	kindnames          = []string{"vr-sros", "vr-nokia_sros"}
	defaultCredentials = nodes.NewCredentials("admin", "admin")
)

const (
	vrsrosDefaultType   = "sr-1"
	scrapliPlatformName = "nokia_sros"
	configDirName       = "tftpboot"
	startupCfgFName     = "config.txt"
	licenseFName        = "license.txt"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrSROS)
	}, defaultCredentials)
}

type vrSROS struct {
	nodes.DefaultNode
}

func (s *vrSROS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)
	// set virtualization requirement
	s.HostRequirements.VirtRequired = true
	s.LicensePolicy = types.LicensePolicyWarn

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

	s.Cfg.Cmd = fmt.Sprintf("--trace --connection-mode %s --hostname %s --variant \"%s\"", s.Cfg.Env["CONNECTION_MODE"],
		s.Cfg.ShortName,
		s.Cfg.NodeType,
	)

	return nil
}

func (s *vrSROS) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return createVrSROSFiles(s)
}

func (s *vrSROS) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	if isPartialConfigFile(s.Cfg.StartupConfig) {
		log.Infof("Waiting for %s to boot and apply config from %s", s.Cfg.LongName, s.Cfg.StartupConfig)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		err := s.applyPartialConfig(ctx, s.Cfg.MgmtIPv4Address, scrapliPlatformName,
			defaultCredentials.GetUsername(), defaultCredentials.GetPassword(),
			s.Cfg.StartupConfig,
		)
		if err != nil {
			return err
		}

		log.Infof("%s: configuration applied", s.Cfg.LongName)
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

// CheckInterfaceName checks if a name of the interface referenced in the topology file correct.
func (s *vrSROS) CheckInterfaceName() error {
	// vsim doesn't seem to support >20 interfaces, yet we allow to set max if number 32 just in case.
	// https://regex101.com/r/bx6kzM/1
	ifRe := regexp.MustCompile(`eth([1-9]|[12][0-9]|3[0-2])$`)
	for _, e := range s.Endpoints {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("nokia SR OS interface name %q doesn't match the required pattern. SR OS interfaces should be named as ethX, where X is from 1 to 32", e.GetIfaceName())
		}
	}

	return nil
}

func createVrSROSFiles(node nodes.Node) error {
	nodeCfg := node.Config()

	// use default startup config load function if config in full form is provided
	if !isPartialConfigFile(nodeCfg.StartupConfig) {
		nodes.LoadStartupConfigFileVr(node, configDirName, startupCfgFName)
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
func (s *vrSROS) applyPartialConfig(ctx context.Context, addr, platformName, username, password string, configFile string) error {
	var err error
	var d *network.Driver

	configContent, err := utils.ReadFileContent(configFile)
	if err != nil {
		return err
	}

	// check file contains content, otherwise exit early
	if len(strings.TrimSpace(string(configContent))) == 0 {
		return nil
	}

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

	mr, err := d.SendConfigsFromFile(configFile)
	if err != nil || mr.Failed != nil {
		return fmt.Errorf("failed to apply config; error: %+v %+v", err, mr.Failed)
	}
	// condfig snippets should not have commit command, so we need to commit manually
	r, err := d.SendConfig("commit")
	if err != nil || r.Failed != nil {
		return fmt.Errorf("failed to commit config; error: %+v %+v", err, mr.Failed)
	}

	r, err = d.SendCommand("/admin save")
	if err != nil || r.Failed != nil {
		return fmt.Errorf("failed to persist config; error: %+v %+v", err, mr.Failed)
	}

	return nil
}

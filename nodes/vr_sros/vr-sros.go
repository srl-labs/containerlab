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

	log "github.com/sirupsen/logrus"
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

func (s *vrSROS) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	// if we got a partial config, it's time to load and apply it
	if isPartialConfigFile(s.Cfg.StartupConfig) {
		// read the file
		bcfg, err := utils.ReadFileContent(s.Cfg.StartupConfig)
		if err != nil {
			return err
		}
		// convert []byte to string config
		cfg := string(bcfg)
		// replace \r\n by \n and then split by \n
		cfgSlc := strings.Split(strings.ReplaceAll(cfg, "\r\n", "\n"), "\n")
		// apply config to node
		log.Infof("Applying partial config %s to %s", s.Cfg.StartupConfig, s.Cfg.LongName)
		err = netconf.ApplyConfig(s.Cfg.MgmtIPv4Address, scrapliPlatformName, defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), cfgSlc)
		if err != nil {
			return err
		}
		log.Infof("%s successfully configured", s.Cfg.LongName)
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
	for _, e := range s.Config().Endpoints {
		if !ifRe.MatchString(e.EndpointName) {
			return fmt.Errorf("nokia SR OS interface name %q doesn't match the required pattern. SR OS interfaces should be named as ethX, where X is from 1 to 32", e.EndpointName)
		}
	}

	return nil
}

func createVrSROSFiles(node nodes.Node) error {

	nodeCfg := node.Config()

	// cache the value of the StartupConfig
	startUpConfigCache := nodeCfg.StartupConfig
	// if the config is a partial one, we set StartupConfig to "" such that
	// nodes.LoadStartupConfigFileVr(...) behaves correctly
	// and does not take the config as a full config
	// afterwards we restore the StartupConfig value again.
	if isPartialConfigFile(nodeCfg.StartupConfig) {
		nodeCfg.StartupConfig = ""
	}
	nodes.LoadStartupConfigFileVr(node, configDirName, startupCfgFName)

	// restore the cached value
	nodeCfg.StartupConfig = startUpConfigCache

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

// isPartialConfigFile returns true if the provided config is a partial config
func isPartialConfigFile(c string) bool {
	return strings.Contains(c, ".partial.")
}

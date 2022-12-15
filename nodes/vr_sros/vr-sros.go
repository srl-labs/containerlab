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

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vr-sros", "vr-nokia_sros"}

const (
	vrsrosDefaultType   = "sr-1"
	scrapliPlatformName = "nokia_sros"
	defaultUser         = "admin"
	defaultPassword     = "admin"
	configDirName       = "tftpboot"
	startupCfgFName     = "config.txt"
	licenseFName        = "license.txt"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(vrSROS)
	})
	err := nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
	if err != nil {
		log.Error(err)
	}
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
	if s.Cfg.StartupConfig == "" {
		s.Cfg.StartupConfig = nodes.DefaultConfigTemplates[s.Cfg.Kind]
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

func (s *vrSROS) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return createVrSROSFiles(s)
}

func (s *vrSROS) SaveConfig(_ context.Context) error {
	err := netconf.SaveConfig(s.Cfg.LongName,
		defaultUser,
		defaultPassword,
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
	nodes.LoadStartupConfigFileVr(node, configDirName, startupCfgFName)

	nodeCfg := node.Config()

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

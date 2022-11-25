// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_nxos

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/cloudflare/cfssl/log"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vr-nxos", "vr-cisco_nxos"}

const (
	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"

	defaultUser     = "admin"
	defaultPassword = "admin"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(vrNXOS)
	})
	nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
}

type vrNXOS struct {
	nodes.DefaultNode
}

func (s *vrNXOS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultUser,
		"PASSWORD":           defaultPassword,
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"VCPU":               "2",
		"RAM":                "4096",
		"DOCKER_NET_V4_ADDR": s.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.Mgmt.IPv6Subnet,
	}
	s.Cfg.Env = utils.MergeStringMaps(defEnv, s.Cfg.Env)

	// mount config dir to support startup-config functionality
	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(path.Join(s.Cfg.LabDir, configDirName), ":/config"))

	s.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		s.Cfg.Env["USERNAME"], s.Cfg.Env["PASSWORD"], s.Cfg.ShortName, s.Cfg.Env["CONNECTION_MODE"])

	// set virtualization requirement
	s.Cfg.HostRequirements.VirtRequired = true

	return nil
}

func (s *vrNXOS) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return loadStartupConfigFile(s.Cfg)
}

func loadStartupConfigFile(node *types.NodeConfig) error {
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(node.LabDir, configDirName), 0777)

	if node.StartupConfig != "" {
		// dstCfg is a path to a file on the clab host that will have rendered configuration
		dstCfg := filepath.Join(node.LabDir, configDirName, startupCfgFName)

		c, err := os.ReadFile(node.StartupConfig)
		if err != nil {
			return err
		}

		cfgTemplate := string(c)

		err = node.GenerateConfig(dstCfg, cfgTemplate)
		if err != nil {
			log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
		}
	}
	return nil
}

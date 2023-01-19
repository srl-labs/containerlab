// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_vqfx

import (
	"context"
	"fmt"
	"path"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/netconf"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	kindnames          = []string{"vr-vqfx", "vr-juniper_vqfx"}
	defaultCredentials = nodes.NewCredentials("admin", "admin@123")
)

const (
	scrapliPlatformName = "juniper_junos"

	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(vrVQFX)
	}, defaultCredentials, nil)
}

type vrVQFX struct {
	nodes.DefaultNode
}

func (s *vrVQFX) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)
	// set virtualization requirement
	s.HostRequirements.VirtRequired = true

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultCredentials.GetUsername(),
		"PASSWORD":           defaultCredentials.GetPassword(),
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"DOCKER_NET_V4_ADDR": s.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.Mgmt.IPv6Subnet,
	}
	s.Cfg.Env = utils.MergeStringMaps(defEnv, s.Cfg.Env)

	// mount config dir to support startup-config functionality
	s.Cfg.Binds = append(s.Cfg.Binds, fmt.Sprint(path.Join(s.Cfg.LabDir, configDirName), ":/config"))

	if s.Cfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		s.Cfg.Binds = append(s.Cfg.Binds, "/dev:/dev")
	}

	s.Cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(), s.Cfg.ShortName, s.Cfg.Env["CONNECTION_MODE"])

	return nil
}

func (s *vrVQFX) PreDeploy(_ context.Context, _, _, _ string) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	return nodes.LoadStartupConfigFileVr(s, configDirName, startupCfgFName)
}

func (s *vrVQFX) SaveConfig(_ context.Context) error {
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
func (n *vrVQFX) CheckInterfaceName() error {
	// allow eth and et interfaces
	// https://regex101.com/r/C3Fhr0/1
	ifRe := regexp.MustCompile(`eth[1-9]+$`)
	for _, e := range n.Config().Endpoints {
		if !ifRe.MatchString(e.EndpointName) {
			return fmt.Errorf("%q interface name %q doesn't match the required pattern. It should be named as ethX, where X is >1", n.Cfg.ShortName, e.EndpointName)
		}
	}

	return nil
}

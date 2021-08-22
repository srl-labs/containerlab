// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/transport"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	// defined env vars for the ceos
	ceosEnv = map[string]string{
		"CEOS":                                "1",
		"EOS_PLATFORM":                        "ceoslab",
		"container":                           "docker",
		"ETBA":                                "4",
		"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT": "1",
		"INTFTYPE":                            "eth",
		"MAPETH0":                             "1",
		"MGMT_INTF":                           "eth0",
	}

	//go:embed ceos.cfg
	cfgTemplate string

	saveCmd = []string{"Cli", "-p", "15", "-c", "copy running flash:conf-saved.conf"}
)

func init() {
	nodes.Register(nodes.NodeKindCEOS, func() nodes.Node {
		return new(ceos)
	})
}

type ceos struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *ceos) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}

	s.cfg.Env = utils.MergeStringMaps(ceosEnv, s.cfg.Env)

	// the node.Cmd should be aligned with the environment.
	var envSb strings.Builder
	envSb.WriteString("/sbin/init ")
	for k, v := range s.cfg.Env {
		envSb.WriteString("systemd.setenv=" + k + "=" + v + " ")
	}
	s.cfg.Cmd = envSb.String()
	s.cfg.MacAddress = utils.GenMac("00:1c:73")

	// mount config dir
	cfgPath := filepath.Join(s.cfg.LabDir, "flash")
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprintf("%s:/mnt/flash/", cfgPath))
	return nil
}

func (s *ceos) Config() *types.NodeConfig { return s.cfg }

func (s *ceos) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return createCEOSFiles(s.cfg)
}

func (s *ceos) Deploy(ctx context.Context) error {
	_, err := s.runtime.CreateContainer(ctx, s.cfg)
	return err
}

func (s *ceos) PostDeploy(ctx context.Context, ns map[string]nodes.Node) error {
	log.Infof("Running postdeploy actions for Arista cEOS '%s' node", s.cfg.ShortName)
	return ceosPostDeploy(ctx, s.runtime, s.cfg)
}

func (s *ceos) WithMgmtNet(*types.MgmtNet)             {}
func (s *ceos) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *ceos) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (s *ceos) SaveConfig(ctx context.Context) error {
	_, stderr, err := s.runtime.Exec(ctx, s.cfg.LongName, saveCmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", s.cfg.ShortName, err)
	}

	if len(stderr) > 0 {
		return fmt.Errorf("%s errors: %s", s.cfg.ShortName, string(stderr))
	}

	confPath := s.cfg.LabDir + "/flash/conf-saved.conf"
	log.Infof("saved cEOS configuration from %s node to %s\n", s.cfg.ShortName, confPath)

	return nil
}

func createCEOSFiles(node *types.NodeConfig) error {
	// generate config directory
	utils.CreateDirectory(path.Join(node.LabDir, "flash"), 0777)
	cfg := filepath.Join(node.LabDir, "flash", "startup-config")
	node.ResStartupConfig = cfg

	// use startup config file provided by a user
	if node.StartupConfig != "" {
		c, err := os.ReadFile(node.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err := node.GenerateConfig(node.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}

	// sysmac is a system mac that is +1 to Ma0 mac
	m, err := net.ParseMAC(node.MacAddress)
	if err != nil {
		return err
	}
	m[5] = m[5] + 1
	utils.CreateFile(path.Join(node.LabDir, "flash", "system_mac_address"), m.String())
	return nil
}

// ceosPostDeploy runs postdeploy actions which are required for ceos nodes
func ceosPostDeploy(ctx context.Context, r runtime.ContainerRuntime, node *types.NodeConfig) error {
	// TODO: implement for ctr (containerd)
	execCmd := "docker"
	openCmd := []string{"exec", "-it"}

	d, err := core.NewCoreDriver(
		node.LongName,
		"arista_eos",
		base.WithAuthBypass(true),
		// disable transport timeout
		base.WithTimeoutTransport(0),
	)
	if err != nil {
		return err
	}

	t, _ := d.Transport.(*transport.System)
	t.ExecCmd = execCmd
	t.OpenCmd = append(openCmd, node.LongName, "Cli")

	fmt.Println(t.ExecCmd, t.OpenCmd)

	transportReady := false
	for !transportReady {
		if err := d.Open(); err != nil {
			log.Debugf("%s - Cli not ready (%s) - waiting.", node.LongName, err)
			time.Sleep(time.Second * 2)
			continue
		}
		transportReady = true
		log.Debugf("%s - Cli ready.", node.LongName)
	}

	cfgs := []string{
		"interface management 0",
		"no ip address",
		"no ipv6 address",
	}

	// adding ipv4 address to configs
	if node.MgmtIPv4Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ip address %s/%d", node.MgmtIPv4Address, node.MgmtIPv4PrefixLength),
		)
	}

	// adding ipv6 address to configs
	if node.MgmtIPv6Address != "" {
		cfgs = append(cfgs,
			fmt.Sprintf("ipv6 address %s/%d", node.MgmtIPv6Address, node.MgmtIPv6PrefixLength),
		)
	}

	// add save to startup cmd
	cfgs = append(cfgs, "wr")

	resp, err := d.SendConfigs(cfgs)
	if err != nil {
		return err
	} else if resp.Failed() {
		return errors.New("failed CLI configuration")
	}

	return err
}

func (s *ceos) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (s *ceos) Delete(ctx context.Context) error {
	return s.runtime.DeleteContainer(ctx, s.Config().LongName)
}

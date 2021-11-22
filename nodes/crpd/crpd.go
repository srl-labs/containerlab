// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package crpd

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	//go:embed crpd.cfg
	cfgTemplate string

	//go:embed sshd_config
	sshdCfg string

	saveCmd = []string{"cli", "show", "conf"}
)

func init() {
	nodes.Register(nodes.NodeKindCRPD, func() nodes.Node {
		return new(crpd)
	})
}

type crpd struct {
	cfg     *types.NodeConfig
	runtime runtime.ContainerRuntime
}

func (s *crpd) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}

	// mount config and log dirs
	s.cfg.Binds = append(s.cfg.Binds,
		fmt.Sprint(path.Join(s.cfg.LabDir, "config"), ":/config"),
		fmt.Sprint(path.Join(s.cfg.LabDir, "log"), ":/var/log"),
		// mount sshd_config
		fmt.Sprint(path.Join(s.cfg.LabDir, "config/sshd_config"), ":/etc/ssh/sshd_config"),
	)

	return nil
}
func (s *crpd) Config() *types.NodeConfig { return s.cfg }

func (s *crpd) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return createCRPDFiles(s.cfg)
}

func (s *crpd) Deploy(ctx context.Context) error {
	_, err := s.runtime.CreateContainer(ctx, s.cfg)
	return err
}

func (s *crpd) PostDeploy(ctx context.Context, _ map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for CRPD %q node", s.cfg.ShortName)
	_, stderr, err := s.runtime.Exec(ctx, s.cfg.ContainerID, []string{"service", "ssh", "restart"})
	if err != nil {
		return err
	}

	if len(stderr) > 0 {
		return fmt.Errorf("crpd post-deploy failed: %s", string(stderr))
	}

	return err
}

func (s *crpd) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (*crpd) WithMgmtNet(*types.MgmtNet)               {}
func (s *crpd) WithRuntime(r runtime.ContainerRuntime) { s.runtime = r }
func (s *crpd) GetRuntime() runtime.ContainerRuntime   { return s.runtime }

func (s *crpd) Delete(ctx context.Context) error {
	return s.runtime.DeleteContainer(ctx, s.Config().LongName)
}

func (s *crpd) SaveConfig(ctx context.Context) error {
	stdout, stderr, err := s.runtime.Exec(ctx, s.cfg.LongName, saveCmd)
	if err != nil {
		return fmt.Errorf("%s: failed to execute cmd: %v", s.cfg.ShortName, err)
	}

	if len(stderr) > 0 {
		return fmt.Errorf("%s errors: %s", s.cfg.ShortName, string(stderr))
	}

	// path by which to save a config
	confPath := s.cfg.LabDir + "/config/juniper.conf"
	err = ioutil.WriteFile(confPath, stdout, 0777)
	if err != nil {
		return fmt.Errorf("failed to write config by %s path from %s container: %v", confPath, s.cfg.ShortName, err)
	}
	log.Infof("saved cRPD configuration from %s node to %s\n", s.cfg.ShortName, confPath)

	return nil
}

///

func createCRPDFiles(nodeCfg *types.NodeConfig) error {
	// create config and logs directory that will be bind mounted to crpd
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, "config"), 0777)
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, "log"), 0777)

	// copy crpd config from default template or user-provided conf file
	cfg := filepath.Join(nodeCfg.LabDir, "/config/juniper.conf")

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err := nodeCfg.GenerateConfig(cfg, cfgTemplate)
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", nodeCfg.ShortName, err)
	}

	// write crpd sshd conf file to crpd node dir
	dst := filepath.Join(nodeCfg.LabDir, "/config/sshd_config")
	err = utils.CreateFile(dst, sshdCfg)
	if err != nil {
		return fmt.Errorf("failed to write sshd_config file %v", err)
	}
	log.Debug("Writing sshd_config succeeded")

	if nodeCfg.License != "" {
		// copy license file to node specific lab directory
		src := nodeCfg.License
		dst = filepath.Join(nodeCfg.LabDir, "/config/license/safenet/junos_sfnt.lic")
		if err = utils.CopyFile(src, dst, 0644); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}
	return nil
}

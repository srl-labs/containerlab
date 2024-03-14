// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package crpd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	licDir = "/config/license/safenet"
)

var (
	kindnames = []string{"crpd", "juniper_crpd"}
	//go:embed crpd.cfg
	defaultCfgTemplate string

	//go:embed sshd_config
	sshdCfg string

	defaultCredentials = kind_registry.NewCredentials("root", "clab123")

	saveCmd       = "cli show conf"
	sshRestartCmd = "service ssh restart"
)

func Init() {
	kind_registry.KindRegistryInstance.Register(kindnames, func() nodes.Node {
		return new(crpd)
	}, defaultCredentials)
}

type crpd struct {
	nodes.DefaultNode
}

func (s *crpd) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	s.DefaultNode = *nodes.NewDefaultNode(s)

	s.Cfg = cfg
	for _, o := range opts {
		o(s)
	}

	// mount config and log dirs
	s.Cfg.Binds = append(s.Cfg.Binds,
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "config"), ":/config"),
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "log"), ":/var/log"),
		// mount sshd_config
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "config/sshd_config"), ":/etc/ssh/sshd_config"),
	)

	return nil
}

func (s *crpd) PreDeploy(_ context.Context, params *nodes.PreDeployParams) error {
	utils.CreateDirectory(s.Cfg.LabDir, 0777)
	_, err := s.LoadOrGenerateCertificate(params.Cert, params.TopologyName)
	if err != nil {
		return nil
	}
	return createCRPDFiles(s)
}

func (s *crpd) PostDeploy(ctx context.Context, _ *nodes.PostDeployParams) error {
	log.Debugf("Running postdeploy actions for CRPD %q node", s.Cfg.ShortName)

	cmd, _ := exec.NewExecCmdFromString(sshRestartCmd)
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("crpd post-deploy failed: %s", execResult.GetStdErrString())
	}

	return err
}

func (s *crpd) SaveConfig(ctx context.Context) error {
	cmd, _ := exec.NewExecCmdFromString(saveCmd)
	execResult, err := s.RunExec(ctx, cmd)
	if err != nil {
		return err
	}

	if len(execResult.GetStdErrString()) > 0 {
		return fmt.Errorf("crpd post-deploy failed: %s", execResult.GetStdErrString())
	}

	// path by which to save a config
	confPath := s.Cfg.LabDir + "/config/juniper.conf"
	err = os.WriteFile(confPath, execResult.GetStdOutByteSlice(), 0777) // skipcq: GO-S2306
	if err != nil {
		return fmt.Errorf("failed to write config by %s path from %s container: %v", confPath, s.Cfg.ShortName, err)
	}
	log.Infof("saved cRPD configuration from %s node to %s\n", s.Cfg.ShortName, confPath)

	return nil
}

func createCRPDFiles(node nodes.Node) error {
	nodeCfg := node.Config()
	// create config and logs directory that will be bind mounted to crpd
	utils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "config"), 0777)
	utils.CreateDirectory(filepath.Join(nodeCfg.LabDir, "log"), 0777)

	// copy crpd config from default template or user-provided conf file
	cfg := filepath.Join(nodeCfg.LabDir, "/config/juniper.conf")
	var cfgTemplate string

	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	if cfgTemplate == "" {
		cfgTemplate = defaultCfgTemplate
	}

	err := node.GenerateConfig(cfg, cfgTemplate)
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
		dst = filepath.Join(nodeCfg.LabDir, licDir, "junos_sfnt.lic")

		if err := os.MkdirAll(filepath.Join(nodeCfg.LabDir, licDir), 0777); err != nil { // skipcq: GSC-G301
			return err
		}

		if err = utils.CopyFile(src, dst, 0644); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}
	return nil
}

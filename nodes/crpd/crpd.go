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
	"strings"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (

	// licDir is the directory where Junos 22+ expects to find the license file.
	licDir  = "/config/license"
	licFile = "license.lic"

	generateable     = true
	generateIfFormat = "eth%d"

	scrapliPlatformName = "juniper_junos"
	NapalmPlatformName  = "junos"
)

var (
	kindNames = []string{"crpd", "juniper_crpd"}
	//go:embed crpd.cfg
	defaultCfgTemplate string

	//go:embed sshd_config
	sshdCfg string

	defaultCredentials = nodes.NewCredentials("root", "clab123")

	saveCmd       = "cli show conf"
	sshRestartCmd = "service ssh restart"
)

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	generateNodeAttributes := nodes.NewGenerateNodeAttributes(generateable, generateIfFormat)
	platformOpts := &nodes.PlatformAttrs{
		ScrapliPlatformName: scrapliPlatformName,
		NapalmPlatformName:  NapalmPlatformName,
	}

	nrea := nodes.NewNodeRegistryEntryAttributes(defaultCredentials, generateNodeAttributes, platformOpts)

	r.Register(kindNames, func() nodes.Node {
		return new(crpd)
	}, nrea)
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
		fmt.Sprint(filepath.Join(s.Cfg.LabDir, "config", "sshd_config"), ":/etc/ssh/sshd_config"),
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
		// If "ssh: unrecognized service" appears in the output we are probably
		// on Junos >=23.4, where the SSH service was renamed to junos-ssh and
		// is fully managed by MGD
		if strings.Contains(execResult.GetStdErrString(), "ssh: unrecognized service") {
			log.Debug(`Caught "ssh: unrecognized service" error, ignoring`)
		} else {
			return fmt.Errorf("crpd post-deploy sshd restart failed: %s", execResult.GetStdErrString())
		}
	}

	if s.Config().License != "" {
		cmd, _ = exec.NewExecCmdFromString(fmt.Sprintf("cli request system license add %s", filepath.Join(licDir, licFile)))
		execResult, err = s.RunExec(ctx, cmd)
		if err != nil {
			return err
		}

		if len(execResult.GetStdErrString()) > 0 {
			return fmt.Errorf("crpd post-deploy license add failed: %s", execResult.GetStdErrString())
		}
		log.Debugf("crpd post-deploy license add result: %s", execResult.GetStdOutString())
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
	cfg := filepath.Join(nodeCfg.LabDir, "config", "juniper.conf")
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
	// Note: this only applies to older versions of Junos (before 23). In later
	// versions the config file is placed in /var/etc/sshd_config and is owned
	// by MGD.
	dst := filepath.Join(nodeCfg.LabDir, "config", "sshd_config")
	err = utils.CreateFile(dst, sshdCfg)
	if err != nil {
		return fmt.Errorf("failed to write sshd_config file %v", err)
	}
	log.Debug("Writing sshd_config succeeded")

	if nodeCfg.License != "" {
		// copy license file to node specific lab directory
		src := nodeCfg.License
		dst = filepath.Join(nodeCfg.LabDir, licDir, licFile)

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

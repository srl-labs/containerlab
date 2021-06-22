// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package crpd

import (
	"context"
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

const (
	nodeKind = "crpd"
)

func init() {
	nodes.Register(nodeKind, func() nodes.Node {
		return new(crpd)
	})
}

type crpd struct {
	cfg *types.NodeConfig
}

func (s *crpd) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	if s.cfg.Config == "" {
		s.cfg.Config = nodes.DefaultConfigTemplates[s.cfg.Kind]
	}

	// mount config and log dirs
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(path.Join(s.cfg.LabDir, "config"), ":/config"))
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(path.Join(s.cfg.LabDir, "log"), ":/var/log"))
	// mount sshd_config
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(path.Join(s.cfg.LabDir, "config/sshd_config"), ":/etc/ssh/sshd_config"))

	return nil
}
func (s *crpd) Config() *types.NodeConfig { return s.cfg }

func (s *crpd) PreDeploy(configName, labCADir, labCARoot string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return createCRPDFiles(s.cfg)
}

func (s *crpd) Deploy(ctx context.Context, r runtime.ContainerRuntime) error {
	return r.CreateContainer(ctx, s.cfg)
}

func (s *crpd) PostDeploy(ctx context.Context, r runtime.ContainerRuntime, ns map[string]nodes.Node) error {
	log.Debugf("Running postdeploy actions for CRPD %q node", s.cfg.ShortName)
	_, stderr, err := r.Exec(ctx, s.cfg.ContainerID, []string{"service ssh restart"})
	if err != nil {
		return err
	}
	if len(stderr) > 0 {
		return fmt.Errorf("crpd post-deploy failed: %s", string(stderr))
	}
	return err
}

func (s *crpd) WithMgmtNet(*types.MgmtNet) {}

///

func createCRPDFiles(nodeCfg *types.NodeConfig) error {
	// create config and logs directory that will be bind mounted to crpd
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, "config"), 0777)
	utils.CreateDirectory(path.Join(nodeCfg.LabDir, "log"), 0777)

	// copy crpd config from default template or user-provided conf file
	cfg := path.Join(nodeCfg.LabDir, "/config/juniper.conf")

	err := nodeCfg.GenerateConfig(cfg, nodes.DefaultConfigTemplates[nodeCfg.Kind])
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", nodeCfg.ShortName, err)
	}

	// copy crpd sshd conf file to crpd node dir
	src := "/etc/containerlab/templates/crpd/sshd_config"
	dst := path.Join(nodeCfg.LabDir, "/config/sshd_config")
	err = utils.CopyFile(src, dst)
	if err != nil {
		return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
	}
	log.Debugf("CopyFile src %s -> dst %s succeeded\n", src, dst)

	if nodeCfg.License != "" {
		// copy license file to node specific lab directory
		src = nodeCfg.License
		dst = path.Join(nodeCfg.LabDir, "/config/license.conf")
		if err = utils.CopyFile(src, dst); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}
	return nil
}

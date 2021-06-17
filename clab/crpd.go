// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"fmt"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func initCrpdNode(c *CLab, nodeDef *types.NodeDefinition, nodeCfg *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	// node.Config, err = c.configInit(nodeCfg, node.Kind)
	c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	// nodeCfg.Image = c.imageInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Image = c.Config.Topology.GetNodeImage(nodeCfg.ShortName)
	// nodeCfg.Group = c.groupInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Group = c.Config.Topology.GetNodeGroup(nodeCfg.ShortName)
	// nodeCfg.Position = c.positionInitialization(nodeDef, nodeCfg.Kind)
	nodeCfg.Position = c.Config.Topology.GetNodePosition(nodeCfg.ShortName)
	nodeCfg.User = user

	// initialize license file
	// lp, err := c.licenseInit(nodeDef, nodeCfg)
	lp, err := c.Config.Topology.GetNodeLicense(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	nodeCfg.License = lp

	// mount config and log dirs
	nodeCfg.Binds = append(nodeCfg.Binds, fmt.Sprint(path.Join(nodeCfg.LabDir, "config"), ":/config"))
	nodeCfg.Binds = append(nodeCfg.Binds, fmt.Sprint(path.Join(nodeCfg.LabDir, "log"), ":/var/log"))
	// mount sshd_config
	nodeCfg.Binds = append(nodeCfg.Binds, fmt.Sprint(path.Join(nodeCfg.LabDir, "config/sshd_config"), ":/etc/ssh/sshd_config"))

	return err
}

func (c *CLab) createCRPDFiles(node *types.NodeConfig) error {
	// create config and logs directory that will be bind mounted to crpd
	utils.CreateDirectory(path.Join(node.LabDir, "config"), 0777)
	utils.CreateDirectory(path.Join(node.LabDir, "log"), 0777)

	// copy crpd config from default template or user-provided conf file
	cfg := path.Join(node.LabDir, "/config/juniper.conf")

	err := node.GenerateConfig(cfg, defaultConfigTemplates[node.Kind])
	if err != nil {
		log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
	}

	// copy crpd sshd conf file to crpd node dir
	src := "/etc/containerlab/templates/crpd/sshd_config"
	dst := node.LabDir + "/config/sshd_config"
	err = copyFile(src, dst)
	if err != nil {
		return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
	}
	log.Debugf("CopyFile src %s -> dst %s succeeded\n", src, dst)

	if node.License != "" {
		// copy license file to node specific lab directory
		src = node.License
		dst = path.Join(node.LabDir, "/config/license.conf")
		if err = copyFile(src, dst); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)
	}
	return err
}

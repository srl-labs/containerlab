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

func initSROSNode(c *CLab, nodeDef *types.NodeDefinition, nodeCfg *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	nodeCfg.Image = c.Config.Topology.GetNodeImage(nodeCfg.ShortName)
	nodeCfg.Group = c.Config.Topology.GetNodeGroup(nodeCfg.ShortName)
	nodeCfg.Position = c.Config.Topology.GetNodePosition(nodeCfg.ShortName)
	nodeCfg.User = user

	// vr-sros type sets the vrnetlab/sros variant (https://github.com/hellt/vrnetlab/sros)
	nodeCfg.NodeType = c.Config.Topology.GetNodeType(nodeCfg.ShortName)
	if nodeCfg.NodeType == "" {
		nodeCfg.NodeType = vrsrosDefaultType
	}
	// initialize license file
	nodeCfg.License, err = c.Config.Topology.GetNodeLicense(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"CONNECTION_MODE":    vrDefConnMode,
		"DOCKER_NET_V4_ADDR": c.Config.Mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": c.Config.Mgmt.IPv6Subnet,
	}
	nodeCfg.Env = utils.MergeStringMaps(defEnv, envs)

	// mount tftpboot dir
	nodeCfg.Binds = append(nodeCfg.Binds, fmt.Sprint(path.Join(nodeCfg.LabDir, "tftpboot"), ":/tftpboot"))
	if nodeCfg.Env["CONNECTION_MODE"] == "macvtap" {
		// mount dev dir to enable macvtap
		nodeCfg.Binds = append(nodeCfg.Binds, "/dev:/dev")
	}

	nodeCfg.Cmd = fmt.Sprintf("--trace --connection-mode %s --hostname %s --variant \"%s\"", nodeCfg.Env["CONNECTION_MODE"],
		nodeCfg.ShortName,
		nodeCfg.NodeType,
	)
	return err
}

func (c *CLab) createVrSROSFiles(node *types.NodeConfig) error {
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(node.LabDir, "tftpboot"), 0777)

	if node.License != "" {
		// copy license file to node specific lab directory
		src := node.License
		dst := path.Join(node.LabDir, "/tftpboot/license.txt")
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("file copy [src %s -> dst %s] failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

		cfg := path.Join(node.LabDir, "tftpboot", "config.txt")
		if node.Config != "" {
			err := node.GenerateConfig(cfg, defaultConfigTemplates[node.Kind])
			if err != nil {
				log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
			}
		} else {
			log.Debugf("Config file exists for node %s", node.ShortName)
		}
	}
	return nil
}

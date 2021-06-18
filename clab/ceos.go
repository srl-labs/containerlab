// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func ceosPostDeploy(ctx context.Context, c *CLab, node *types.NodeConfig, lworkers uint) error {
	// regenerate ceos config since it is now known which IP address docker assigned to this container
	err := node.GenerateConfig(node.ResConfig, defaultConfigTemplates[node.Kind])
	if err != nil {
		return err
	}
	log.Infof("Restarting '%s' node", node.ShortName)
	// force stopping and start is faster than ContainerRestart
	var timeout time.Duration = 1
	err = c.Runtime.StopContainer(ctx, node.ContainerID, &timeout)
	if err != nil {
		return err
	}
	// remove the netns symlink created during original start
	// we will re-symlink it later
	if err := deleteNetnsSymlink(node.LongName); err != nil {
		return err
	}
	err = c.Runtime.StartContainer(ctx, node.ContainerID)
	if err != nil {
		return err
	}
	node.NSPath, err = c.Runtime.GetNSPath(ctx, node.ContainerID)
	if err != nil {
		return err
	}
	err = utils.LinkContainerNS(node.NSPath, node.LongName)
	if err != nil {
		return err
	}

	return err
}

func initCeosNode(c *CLab, nodeDef *types.NodeDefinition, nodeCfg *types.NodeConfig, user string, envs map[string]string) error {
	var err error

	// initialize the global parameters with defaults, can be overwritten later
	// node.Config, err = c.configInit(nodeCfg, node.Kind)
	c.Config.Topology.GetNodeConfig(nodeCfg.ShortName)
	if err != nil {
		return err
	}
	nodeCfg.Image = c.Config.Topology.GetNodeImage(nodeCfg.ShortName)
	nodeCfg.Position = c.Config.Topology.GetNodePosition(nodeCfg.ShortName)

	// initialize specific container information

	// defined env vars for the ceos
	kindEnv := map[string]string{
		"CEOS":                                "1",
		"EOS_PLATFORM":                        "ceoslab",
		"container":                           "docker",
		"ETBA":                                "4",
		"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT": "1",
		"INTFTYPE":                            "eth",
		"MAPETH0":                             "1",
		"MGMT_INTF":                           "eth0"}
	nodeCfg.Env = utils.MergeStringMaps(kindEnv, envs)

	// the node.Cmd should be aligned with the environment.
	var envSb strings.Builder
	envSb.WriteString("/sbin/init ")
	for k, v := range nodeCfg.Env {
		envSb.WriteString("systemd.setenv=" + k + "=" + v + " ")

	}
	nodeCfg.Cmd = envSb.String()

	nodeCfg.User = user
	nodeCfg.Group = c.Config.Topology.GetNodeGroup(nodeCfg.ShortName)
	nodeCfg.NodeType = nodeDef.Type

	nodeCfg.MacAddress = genMac("00:1c:73")

	// mount config dir
	cfgPath := filepath.Join(nodeCfg.LabDir, "flash")
	nodeCfg.Binds = append(nodeCfg.Binds, fmt.Sprint(cfgPath, ":/mnt/flash/"))

	return err
}

func (c *CLab) createCEOSFiles(node *types.NodeConfig) error {
	// generate config directory
	utils.CreateDirectory(path.Join(node.LabDir, "flash"), 0777)
	cfg := path.Join(node.LabDir, "flash", "startup-config")
	node.ResConfig = cfg

	// sysmac is a system mac that is +1 to Ma0 mac
	m, err := net.ParseMAC(node.MacAddress)
	if err != nil {
		return err
	}
	m[5] = m[5] + 1
	createFile(path.Join(node.LabDir, "flash", "system_mac_address"), m.String())
	return nil
}

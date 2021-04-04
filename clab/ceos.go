package clab

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

func ceosPostDeploy(ctx context.Context, c *CLab, node *Node, lworkers uint) error {
	// regenerate ceos config since it is now known which IP address docker assigned to this container
	err := node.generateConfig(node.ResConfig)
	if err != nil {
		return err
	}
	log.Infof("Restarting '%s' node", node.ShortName)
	// force stopping and start is faster than ContainerRestart
	var timeout time.Duration = 1
	err = c.DockerClient.ContainerStop(ctx, node.ContainerID, &timeout)
	if err != nil {
		return err
	}
	// remove the netns symlink created during original start
	// we will re-symlink it later
	if err := deleteNetnsSymlink(node.LongName); err != nil {
		return err
	}
	err = c.DockerClient.ContainerStart(ctx, node.ContainerID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	// since container has been restarted, we need to get its new NSPath and link netns
	cont, err := c.DockerClient.ContainerInspect(ctx, node.ContainerID)
	if err != nil {
		return err
	}
	log.Debugf("node %s new pid %v", node.LongName, cont.State.Pid)
	node.NSPath = "/proc/" + strconv.Itoa(cont.State.Pid) + "/ns/net"
	err = linkContainerNS(node.NSPath, node.LongName)
	if err != nil {
		return err
	}

	return err
}

func initCeosNode(c *CLab, nodeCfg NodeConfig, node *Node, user string, envs map[string]string) error {
	var err error

	// initialize the global parameters with defaults, can be overwritten later
	node.Config, err = c.configInit(&nodeCfg, node.Kind)
	if err != nil {
		return err
	}
	node.Image = c.imageInitialization(&nodeCfg, node.Kind)
	node.Position = c.positionInitialization(&nodeCfg, node.Kind)

	// initialize specific container information
	node.Cmd = "/sbin/init systemd.setenv=INTFTYPE=eth systemd.setenv=ETBA=4 systemd.setenv=SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1 systemd.setenv=CEOS=1 systemd.setenv=EOS_PLATFORM=ceoslab systemd.setenv=container=docker systemd.setenv=MAPETH0=1 systemd.setenv=MGMT_INTF=eth0"

	// defined env vars for the ceos
	kindEnv := map[string]string{
		"CEOS":                                "1",
		"EOS_PLATFORM":                        "ceoslab",
		"container":                           "docker",
		"ETBA":                                "1",
		"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT": "1",
		"INTFTYPE":                            "eth",
		"MAPETH0":                             "1",
		"MGMT_INTF":                           "eth0"}
	node.Env = mergeStringMaps(kindEnv, envs)

	node.User = user
	node.Group = c.groupInitialization(&nodeCfg, node.Kind)
	node.NodeType = nodeCfg.Type

	node.MacAddress = genMac("00:1c:73")

	// mount config dir
	cfgPath := filepath.Join(node.LabDir, "flash")
	node.Binds = append(node.Binds, fmt.Sprint(cfgPath, ":/mnt/flash/"))

	return err
}

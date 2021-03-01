package clab

import (
	"context"
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

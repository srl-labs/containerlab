package clab

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/runtime/ignite"
)

// Destroy the given topology.
func (c *CLab) Destroy(ctx context.Context, maxWorkers uint, keepMgmtNet bool) error {
	containers, err := c.ListNodesContainersIgnoreNotFound(ctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return nil
	}

	if maxWorkers == 0 {
		maxWorkers = uint(len(c.Nodes))
	}

	// a set of workers that do not support concurrency
	serialNodes := make(map[string]struct{})
	for _, n := range c.Nodes {
		if n.GetRuntime().GetName() == ignite.RuntimeName {
			serialNodes[n.Config().LongName] = struct{}{}
			// decreasing the num of maxWorkers as they are used for concurrent nodes
			maxWorkers = maxWorkers - 1
		}
	}

	// Serializing ignite workers due to busy device error
	if _, ok := c.Runtimes[ignite.RuntimeName]; ok {
		maxWorkers = 1
	}

	log.Info("Destroying lab", "name", c.Config.Name)
	c.deleteNodes(ctx, maxWorkers, serialNodes)

	c.deleteToolContainers(ctx)

	log.Info("Removing host entries", "path", "/etc/hosts")
	err = c.DeleteEntriesFromHostsFile()
	if err != nil {
		return fmt.Errorf("error while trying to clean up the hosts file: %w", err)
	}

	log.Info("Removing SSH config", "path", c.TopoPaths.SSHConfigPath())
	err = c.RemoveSSHConfig(c.TopoPaths)
	if err != nil {
		log.Errorf("failed to remove ssh config file: %v", err)
	}

	// delete container network namespaces symlinks
	for _, node := range c.Nodes {
		err = node.DeleteNetnsSymlink()
		if err != nil {
			return fmt.Errorf("error while deleting netns symlinks: %w", err)
		}
	}

	// delete lab management network
	if c.Config.Mgmt.Network != "bridge" && !keepMgmtNet {
		log.Debugf("Calling DeleteNet method. *CLab.Config.Mgmt value is: %+v", c.Config.Mgmt)
		if err = c.globalRuntime().DeleteNet(ctx); err != nil {
			// do not log error message if deletion error simply says that such network doesn't exist
			if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
				log.Error(err)
			}
		}
	}
	return nil
}

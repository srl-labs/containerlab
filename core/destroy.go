package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	containerlablabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
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

func (c *CLab) deleteNodes(ctx context.Context, workers uint, serialNodes map[string]struct{}) {
	wg := new(sync.WaitGroup)

	concurrentChan := make(chan nodes.Node)
	serialChan := make(chan nodes.Node)

	workerFunc := func(i uint, input chan nodes.Node, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case n := <-input:
				if n == nil {
					log.Debugf("Worker %d terminating...", i)
					return
				}
				err := n.Delete(ctx)
				if err != nil {
					log.Errorf("could not remove container %q: %v", n.Config().LongName, err)
				}
			case <-ctx.Done():
				return
			}
		}
	}

	// start concurrent workers
	wg.Add(int(workers))

	for i := range workers {
		go workerFunc(i, concurrentChan, wg)
	}

	// start the serial worker
	if len(serialNodes) > 0 {
		wg.Add(1)
		go workerFunc(workers, serialChan, wg)
	}

	// send nodes to workers
	for _, n := range c.Nodes {
		if _, ok := serialNodes[n.Config().LongName]; ok {
			serialChan <- n
			continue
		}
		concurrentChan <- n
	}

	// close channel to terminate the workers
	close(concurrentChan)
	close(serialChan)

	// also call delete on the special nodes
	for _, n := range c.getSpecialLinkNodes() {
		err := n.Delete(ctx)
		if err != nil {
			log.Warn(err)
		}
	}

	wg.Wait()
}

func (c *CLab) deleteToolContainers(ctx context.Context) {
	toolTypes := []string{"sshx", "gotty"}

	for _, toolType := range toolTypes {
		toolFilter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      containerlablabels.ToolType,
				Operator:   "=",
				Match:      toolType,
			},
			{
				FilterType: "label",
				Field:      containerlablabels.Containerlab,
				Operator:   "=",
				Match:      c.Config.Name,
			},
		}

		containers, err := c.globalRuntime().ListContainers(ctx, toolFilter)
		if err != nil {
			log.Error("Failed to list tool containers", "tool", toolType, "error", err)
			return
		}

		if len(containers) == 0 {
			log.Debug("No tool containers found for lab", "tool", toolType, "lab", c.Config.Name)
			return
		}

		log.Info("Found tool containers associated with a lab", "tool", toolType, "lab",
			c.Config.Name, "count", len(containers))

		for _, container := range containers {
			containerName := strings.TrimPrefix(container.Names[0], "/")
			log.Info("Removing tool container", "tool", toolType, "container", containerName)
			if err := c.globalRuntime().DeleteContainer(ctx, containerName); err != nil {
				log.Error("Failed to remove tool container", "tool", toolType,
					"container", containerName, "error", err)
			} else {
				log.Info("Tool container removed successfully", "tool", toolType, "container", containerName)
			}
		}
	}
}

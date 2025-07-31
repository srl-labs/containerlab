package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/labels"
	containerlablabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

func (c *CLab) DestroyNew(ctx context.Context, options ...DestroyOption) error {
	opts := NewDestroyOptions()

	for _, opt := range options {
		opt(opts)
	}

	var containers []runtime.GenericContainer

	var err error

	// TODO this listing logic is obviously super redundant throughout clab and should be clearer
	// and simplified. and maybe more automatic. like the clab instance will know if it has a name
	// or filters or w/e set
	if opts.all {
		containers, err = c.ListContainers(ctx, nil)
	} else if common.Topo != "" {
		containers, err = c.ListNodesContainersIgnoreNotFound(ctx)
	} else {
		// TODO murder kill common entirely. these can/should be things passed to a clab instance
		// where we are not importing like this (even in cmd it just feels lazy)
		// List containers based on labels (--name or --all)
		var gLabels []*types.GenericFilter
		if common.Name != "" {
			// Filter by specific lab name
			gLabels = []*types.GenericFilter{{
				FilterType: "label", Match: common.Name,
				Field: labels.Containerlab, Operator: "=",
			}}
		} else { // --all case
			// Filter for any containerlab container
			gLabels = []*types.GenericFilter{{
				FilterType: "label",
				Field:      labels.Containerlab, Operator: "exists",
			}}
		}
		containers, err = c.ListContainers(ctx, gLabels)
	}

	if err != nil {
		return err
	}

	// TOOD second thing can be nil, but these are same/same when -t flag used
	// fmt.Println("COMMON.TOPO >>> ", common.Topo)
	// fmt.Println("C.TOPOPATHS.TOPOLIGYFILENAME >>> ", c.TopoPaths.TopologyFilenameAbsPath())

	if len(containers) == 0 {
		log.Info("no containerlab containers found")

		if opts.cleanup && !opts.all {
			var labDirs []string

			if common.Topo != "" {
				topoDir := filepath.Dir(common.Topo)
				log.Debug("Looking for lab directory next to topology file", "path", topoDir)
				labDirs, _ = filepath.Glob(filepath.Join(topoDir, "clab-*"))
			} else if len(labDirs) == 0 {
				log.Debug("Looking for lab directory in current directory")
				labDirs, _ = filepath.Glob("clab-*")
			}

			if len(labDirs) != 0 {
				log.Info("Removing lab directory", "path", labDirs[0])
				if err := os.RemoveAll(labDirs[0]); err != nil {
					log.Errorf("error deleting lab directory: %v", err)
				}
			}
		}

		return nil
	}

	// topo will hold the reference to the topology file
	// as the key and the respective lab directory as the referenced value
	topos := map[string]string{}

	for _, container := range containers {
		topoFile, ok := container.Labels[containerlablabels.TopoFile]
		if !ok {
			continue
		}

		topos[topoFile] = filepath.Dir(container.Labels[containerlablabels.NodeLabDir])
	}

	log.Debugf("got the following topologies for destroy: %+v", topos)

	// if all and cli doesnt have --yes flag and in a terminal, prompt user confirmation
	if opts.all && opts.terminalPrompt && utils.IsTerminal(os.Stdin.Fd()) {
		err := cliPromptToDestroyAll(topos)
		if err != nil {
			return err
		}
	}

	var clabs []*CLab

	for topo, labdir := range topos {
		cOpts := []ClabOption{
			WithTimeout(c.timeout),
			WithTopoPath(topo, common.VarsFile),
			WithNodeFilter(opts.nodeFilter),
			// during destroy we don't want to check bind paths
			// as it is irrelevant for this command.
			WithSkippedBindsPathsCheck(),
			WithRuntime(common.Runtime,
				&runtime.RuntimeConfig{
					Debug:            common.Debug,
					Timeout:          common.Timeout,
					GracefulShutdown: common.Graceful,
				},
			),
		}

		// TODO i think if we copy the runtimes this should be already set for us...?
		if opts.keepMgmtNet {
			cOpts = append(cOpts, WithKeepMgmtNet())
		}

		if common.Name != "" {
			cOpts = append(cOpts, WithLabName(common.Name))
		}

		log.Debugf(
			"going through extracted topos for destroy, got topo file %v and generated opts %+v",
			topo,
			cOpts,
		)

		clab, err := NewContainerLab(cOpts...)
		if err != nil {
			return err
		}

		clab.Config.Debug = c.Config.Debug

		// check if labdir exists and is a directory
		if labdir != "" && utils.FileOrDirExists(labdir) {
			// adjust the labdir. Usually we take the PWD. but now on destroy time,
			// we might be in a different Dir.
			err = clab.TopoPaths.SetLabDir(labdir)
			if err != nil {
				return err
			}
		}

		err = links.SetMgmtNetUnderlyingBridge(clab.Config.Mgmt.Bridge)
		if err != nil {
			return err
		}

		// create management network or use existing one
		// we call this to populate the nc.cfg.mgmt.bridge variable
		// which is needed for the removal of the iptables rules
		err = clab.CreateNetwork(ctx)
		if err != nil {
			return err
		}

		err = clab.ResolveLinks()
		if err != nil {
			return err
		}

		clabs = append(clabs, clab)
	}

	var errs []error

	for _, clab := range clabs {
		// TODO this method can probably become "Destroy" and the current one just goes to some
		// private internal method i assume...?
		err = clab.Destroy(ctx, opts.maxWorkers, opts.keepMgmtNet)
		if err != nil {
			log.Errorf("Error occurred during the %s lab deletion: %v", clab.Config.Name, err)
			errs = append(errs, err)
		}

		// TODO see note above -- do we just have a defer cleanup (probably cant because of topos
		// not existing yet, unless we var topos way up top which seems reasonable...)
		if opts.cleanup {
			err = os.RemoveAll(clab.TopoPaths.TopologyLabDir())
			if err != nil {
				log.Errorf("error deleting lab directory: %v", err)
			}
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("error(s) occurred during the deletion. Check log messages")
	}

	return nil
}

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

func cliPromptToDestroyAll(topos map[string]string) error {
	var sb strings.Builder

	idx := 1

	for topo, labDir := range topos {
		sb.WriteString(fmt.Sprintf("  %d. Topology: %s\n     Lab Dir: %s\n", idx, topo, labDir))
		idx++
	}

	log.Warn("The following labs will be removed:", "labs", sb.String())

	warningStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")) // red color (ansi code 1)
	prompt := "Are you sure you want to remove all labs listed above? Enter 'y', to confirm or ENTER to abort: "
	fmt.Print(warningStyle.Render(prompt))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %v", err)
	}

	answer := strings.TrimSpace(input)
	if answer == "" {
		return errors.New("aborted by the user. No labs were removed")
	}

	answer = strings.ToLower(answer)
	if answer != "y" && answer != "yes" {
		return errors.New("aborted by the user. No labs were removed")
	}

	return nil
}

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
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabruntimeignite "github.com/srl-labs/containerlab/runtime/ignite"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func (c *CLab) Destroy(ctx context.Context, options ...DestroyOption) (err error) {
	opts := NewDestroyOptions()

	for _, opt := range options {
		opt(opts)
	}

	var containers []clabruntime.GenericContainer

	switch {
	case opts.all:
		containers, err = c.ListContainers(ctx)
	case c.TopoPaths.TopologyFilenameAbsPath() != "":
		containers, err = c.ListNodesContainersIgnoreNotFound(ctx)
	default:
		var listOpts []ListOption

		if c.Config.Name != "" {
			listOpts = []ListOption{WithListLabName(c.Config.Name)}
		} else {
			listOpts = []ListOption{WithListclabLabelExists()}
		}

		containers, err = c.ListContainers(ctx, listOpts...)
	}

	if err != nil {
		return err
	}

	// topo will hold the reference to the topology file
	// as the key and the respective lab directory as the referenced value
	topos := map[string]string{}

	for idx := range containers {
		topoFile, ok := containers[idx].Labels[clabconstants.TopoFile]
		if !ok {
			continue
		}

		topos[topoFile] = filepath.Dir(containers[idx].Labels[clabconstants.NodeLabDir])
	}

	defer func() {
		if opts.cleanup {
			err = c.destroyLabDirs(topos, opts.all)
		}
	}()

	if len(topos) == 0 {
		return nil
	}

	log.Debugf("got the following topologies for destroy: %+v", topos)

	// if all, and cli doesnt have --yes flag, and in a terminal -- prompt user confirmation
	if opts.all && opts.terminalPrompt && clabutils.IsTerminal(os.Stdin.Fd()) {
		err := cliPromptToDestroyAll(topos)
		if err != nil {
			return err
		}
	}

	var errs []error

	for topo, labDir := range topos {
		cc, err := c.makeCopyForDestroy(ctx, topo, labDir, *opts)
		if err != nil {
			log.Errorf("error creating clab instance for topo %q: %v", topo, err)

			return err
		}

		err = cc.destroy(ctx, opts.maxWorkers, opts.keepMgmtNet)
		if err != nil {
			log.Errorf("Error occurred during the %s lab deletion: %v", cc.Config.Name, err)
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("error(s) occurred during the deletion. Check log messages")
	}

	return err
}

// creates a mostly cloned version of the current c but set to the new topology, and with the
// necessary steps (mgmt network things) handled preparing the new CLab for destruction.
func (c *CLab) makeCopyForDestroy(
	ctx context.Context,
	topo,
	labDir string,
	opts DestroyOptions,
) (*CLab, error) {
	newOpts := []ClabOption{
		WithTimeout(c.timeout),
		WithTopoPath(topo, c.TopoPaths.VarsFilenameAbsPath()),
		WithNodeFilter(opts.nodeFilter),
		// during destroy we don't want to check bind paths
		// as it is irrelevant for this command.
		WithSkippedBindsPathsCheck(),
		WithRuntime(
			c.globalRuntimeName,
			&clabruntime.RuntimeConfig{
				Debug:            c.Config.Debug,
				Timeout:          c.timeout,
				GracefulShutdown: opts.graceful,
			},
		),
	}

	if opts.keepMgmtNet {
		newOpts = append(newOpts, WithKeepMgmtNet())
	}

	cc, err := NewContainerLab(newOpts...)
	if err != nil {
		return nil, err
	}

	if labDir != "" && clabutils.FileOrDirExists(labDir) {
		// adjust the labdir. Usually we take the PWD. but now on destroy time,
		// we might be in a different Dir.
		err = cc.TopoPaths.SetLabDir(labDir)
		if err != nil {
			return nil, err
		}
	}

	err = clablinks.SetMgmtNetUnderlyingBridge(cc.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	// create management network or use existing one
	// we call this to populate the nc.cfg.mgmt.bridge variable
	// which is needed for the removal of the iptables rules
	err = cc.CreateNetwork(ctx)
	if err != nil {
		return nil, err
	}

	err = cc.ResolveLinks()
	if err != nil {
		return nil, err
	}

	return cc, nil
}

func (c *CLab) destroyLabDirs(topos map[string]string, all bool) error {
	if len(topos) == 0 {
		log.Info("no containerlab containers found")

		if !all {
			var labDirs []string

			topoPath := c.TopoPaths.TopologyFilenameAbsPath()

			if topoPath != "" {
				topoDir := filepath.Dir(topoPath)
				log.Debug("Looking for lab directory next to topology file", "path", topoDir)
				labDirs, _ = filepath.Glob(filepath.Join(topoDir, "clab-*"))
			} else {
				log.Debug("Looking for lab directory in current directory")

				labDirs, _ = filepath.Glob("clab-*")
			}

			if len(labDirs) != 0 {
				log.Info("Removing lab directory", "path", labDirs[0])

				err := os.RemoveAll(labDirs[0])
				if err != nil {
					log.Errorf("error deleting lab directory: %v", err)

					return err
				}
			}
		}

		return nil
	}

	for _, labdir := range topos {
		err := os.RemoveAll(labdir)
		if err != nil {
			log.Errorf("error deleting lab directory: %v", err)
		}
	}

	return nil
}

func (c *CLab) destroy(ctx context.Context, maxWorkers uint, keepMgmtNet bool) error {
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
		if n.GetRuntime().GetName() == clabruntimeignite.RuntimeName {
			serialNodes[n.Config().LongName] = struct{}{}
			// decreasing the num of maxWorkers as they are used for concurrent nodes
			maxWorkers--
		}
	}

	// Serializing ignite workers due to busy device error
	if _, ok := c.Runtimes[clabruntimeignite.RuntimeName]; ok {
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
			switch {
			case err.Error() == fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network):
			case strings.Contains(err.Error(), fmt.Sprintf(
				" network %s not found", c.Config.Mgmt.Network)):
			default:
				log.Error(err)
			}
		}
	}

	return nil
}

func (c *CLab) deleteNodes(ctx context.Context, workers uint, serialNodes map[string]struct{}) {
	wg := new(sync.WaitGroup)

	concurrentChan := make(chan clabnodes.Node)
	serialChan := make(chan clabnodes.Node)

	workerFunc := func(i uint, input chan clabnodes.Node, wg *sync.WaitGroup) {
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
		toolFilter := []*clabtypes.GenericFilter{
			{
				FilterType: "label",
				Field:      clabconstants.ToolType,
				Operator:   "=",
				Match:      toolType,
			},
			{
				FilterType: "label",
				Field:      clabconstants.Containerlab,
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

		for idx := range containers {
			containerName := strings.TrimPrefix(containers[idx].Names[0], "/")

			log.Info("Removing tool container", "tool", toolType, "container", containerName)

			if err := c.globalRuntime().DeleteContainer(ctx, containerName); err != nil {
				log.Error("Failed to remove tool container", "tool", toolType,
					"container", containerName, "error", err)
			} else {
				log.Info(
					"Tool container removed successfully",
					"tool",
					toolType,
					"container",
					containerName,
				)
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

	// red color (ansi code 1)
	warningStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	prompt := "Are you sure you want to remove all labs listed above? Enter 'y'," +
		" to confirm or ENTER to abort: "
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

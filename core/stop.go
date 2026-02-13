package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
	"golang.org/x/sys/unix"
)

// StopNodes stops one or more deployed nodes without losing their dataplane links by parking
// the node's interfaces in a dedicated network namespace before stopping the container.
func (c *CLab) StopNodes(ctx context.Context, nodeNames []string) error {
	nodeNames = resolveLifecycleNodeNames(c.Nodes, nodeNames)
	if len(nodeNames) == 0 {
		return fmt.Errorf("%w: lab has no nodes", claberrors.ErrIncorrectInput)
	}

	return c.withLabLock(func() error {
		if err := c.ResolveLinks(); err != nil {
			return err
		}

		nsProviders := c.namespaceShareProviders()

		for _, nodeName := range nodeNames {
			n, err := c.validateLifecycleNode(ctx, nodeName, nsProviders)
			if err != nil {
				return err
			}

			status := n.GetRuntime().GetContainerStatus(ctx, n.Config().LongName)
			switch status {
			case clabruntime.Running:
				if err := n.Stop(ctx); err != nil {
					return err
				}
			case clabruntime.Stopped:
				log.Debugf("node %q already stopped, skipping", nodeName)
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}
		}

		return nil
	})
}

func resolveLifecycleNodeNames(allNodes map[string]clabnodes.Node, requested []string) []string {
	if len(requested) > 0 {
		return requested
	}

	nodeNames := make([]string, 0, len(allNodes))
	for nodeName := range allNodes {
		nodeNames = append(nodeNames, nodeName)
	}

	slices.Sort(nodeNames)

	return nodeNames
}

func (c *CLab) validateLifecycleNode(
	ctx context.Context,
	nodeName string,
	nsProviders map[string][]string,
) (clabnodes.Node, error) {
	n, ok := c.Nodes[nodeName]
	if !ok {
		return nil, fmt.Errorf("%w: node %q is not present in the topology", claberrors.ErrIncorrectInput, nodeName)
	}

	cfg := n.Config()

	if cfg.IsRootNamespaceBased {
		return nil, fmt.Errorf("node %q is root-namespace based and cannot be stopped/started", nodeName)
	}

	if cfg.AutoRemove {
		return nil, fmt.Errorf("node %q has auto-remove enabled and is not supported", nodeName)
	}

	containers, err := n.GetContainers(ctx)
	if err != nil {
		return nil, err
	}
	if len(containers) != 1 {
		return nil, fmt.Errorf("node %q is not supported (expected 1 container, got %d)", nodeName, len(containers))
	}

	if strings.HasPrefix(cfg.NetworkMode, "container:") {
		return nil, fmt.Errorf("node %q uses network-mode %q and is not supported", nodeName, cfg.NetworkMode)
	}
	if dependers := nsProviders[nodeName]; len(dependers) > 0 {
		return nil, fmt.Errorf(
			"node %q is used as a network-mode provider for %v and is not supported",
			nodeName,
			dependers,
		)
	}

	return n, nil
}

func (c *CLab) namespaceShareProviders() map[string][]string {
	providers := make(map[string][]string)
	for _, n := range c.Nodes {
		netModeArr := strings.SplitN(n.Config().NetworkMode, ":", 2) //nolint:mnd
		if len(netModeArr) != 2 || netModeArr[0] != "container" {
			continue
		}
		ref := netModeArr[1]
		if _, exists := c.Nodes[ref]; !exists {
			continue
		}
		providers[ref] = append(providers[ref], n.Config().ShortName)
	}
	return providers
}

func (c *CLab) withLabLock(f func() error) error {
	lockPath := filepath.Join(c.TopoPaths.TopologyLabDir(), ".clab.lock")
	if c.TopoPaths.TopologyLabDir() == "" || !clabutils.DirExists(c.TopoPaths.TopologyLabDir()) {
		lockPath = c.fallbackLockPath()
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), clabconstants.PermissionsDirDefault); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, clabconstants.PermissionsFileDefault)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

	return f()
}

func (c *CLab) fallbackLockPath() string {
	baseDir := filepath.Join(c.TopoPaths.ClabTmpDir(), "locks")

	input := c.Config.Name
	if c.TopoPaths != nil && c.TopoPaths.TopologyFileIsSet() {
		input = c.TopoPaths.TopologyFilenameAbsPath() + "|" + c.Config.Name
	}

	sum := sha1.Sum([]byte(input))
	suffix := hex.EncodeToString(sum[:])[:10]
	return filepath.Join(baseDir, suffix+".lock")
}

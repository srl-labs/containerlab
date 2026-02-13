package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
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
				log.Debugf("node %q already running, skipping", nodeName)
				continue
			case clabruntime.Stopped:
				// continue below
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if err := c.startNode(ctx, n); err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *CLab) startNode(ctx context.Context, n clabnodes.Node) error {
	cfg := n.Config()

	parkName := parkingNetnsName(cfg.LongName)
	parkPath := filepath.Join("/run/netns", parkName)
	if !clabutils.FileOrDirExists(parkPath) {
		return fmt.Errorf(
			"node %q has no parking netns %q; seamless start requires stopping via containerlab",
			cfg.ShortName,
			parkName,
		)
	}
	parkingNode := clablinks.NewGenericLinkNode(parkName, parkPath)

	if _, err := n.GetRuntime().StartContainer(ctx, cfg.LongName, n); err != nil {
		return fmt.Errorf("node %q failed starting container: %w", cfg.ShortName, err)
	}

	if _, err := n.GetNSPath(ctx); err != nil {
		// Try to keep destroy/inspect operational by repointing back to the parking netns.
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	// Move interfaces back into the container netns.
	moved, err := moveEndpointsBetweenNodes(ctx, parkingNode, n, n.GetEndpoints(), nil)
	if err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_ = rollbackMovedEndpoints(ctx, n, parkingNode, moved, preMoveSetDownOptions())
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed restoring interfaces: %w", cfg.ShortName, err)
	}

	// Bring restored interfaces up.
	if err := setEndpointsUp(ctx, n, moved); err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_ = rollbackMovedEndpoints(ctx, n, parkingNode, moved, preMoveSetDownOptions())
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed enabling interfaces: %w", cfg.ShortName, err)
	}

	// Re-run topology exec commands on lifecycle start to restore node-local interface config
	// (for example IP addresses added during deploy exec phase).
	execCollection := clabexec.NewExecCollection()
	if err := n.RunExecFromConfig(ctx, execCollection); err != nil {
		log.Errorf("failed to run exec commands for node %q on lifecycle start: %v", cfg.ShortName, err)
	}
	execCollection.Log()

	return nil
}

package core

import (
	"context"
	"fmt"

	claberrors "github.com/srl-labs/containerlab/errors"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// RestartNodes performs stop+start for each node, restoring parked interfaces.
func (c *CLab) RestartNodes(ctx context.Context, nodeNames []string) error {
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
			if status == clabruntime.NotFound {
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if status == clabruntime.Running {
				if err := n.Stop(ctx); err != nil {
					return err
				}
			}

			// Start/restore is only meaningful when a node is stopped and its interfaces are parked.
			if err := n.Start(ctx); err != nil {
				return err
			}
		}

		return nil
	})
}

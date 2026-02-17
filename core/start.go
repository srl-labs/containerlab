package core

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabruntime "github.com/srl-labs/containerlab/runtime"
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

			status := n.GetContainerStatus(ctx)
			switch status {
			case clabruntime.Running:
				log.Debugf("node %q already running, skipping", nodeName)
				continue
			case clabruntime.Stopped:
				// continue below
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if err := n.Start(ctx); err != nil {
				return err
			}
		}

		return nil
	})
}

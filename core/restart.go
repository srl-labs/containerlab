package core

import (
	"context"
)

// RestartNodes performs stop+start for each node, restoring parked interfaces.
func (c *CLab) RestartNodes(ctx context.Context, nodeNames []string) error {
	if err := c.ResolveLinks(); err != nil {
		return err
	}

	nodes, err := c.lifecycleNodes(nodeNames)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		if err := n.Stop(ctx); err != nil {
			return err
		}

		if err := n.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

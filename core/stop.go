package core

import (
	"context"
)

// StopNodes stops one or more deployed nodes without losing their dataplane links by parking
// the node's interfaces in a dedicated network namespace before stopping the container.
func (c *CLab) StopNodes(ctx context.Context, nodeNames []string) error {
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
	}

	return nil
}

package core

import (
	"context"
)

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
	if err := c.ResolveLinks(); err != nil {
		return err
	}

	nodes, err := c.lifecycleNodes(nodeNames)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		if err := n.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

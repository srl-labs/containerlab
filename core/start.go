package core

import (
	"context"

	clabtypes "github.com/srl-labs/containerlab/types"
)

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
	if err := c.ResolveLinks(); err != nil {
		return err
	}

	nodes, deps, err := c.lifecycleStartNodes(nodeNames)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		for _, dep := range deps[n.GetShortName()] {
			if dep.Stage != clabtypes.WaitForHealthy {
				continue
			}

			if err := c.waitForLifecycleNodeHealthy(ctx, n.GetShortName(), dep.Node); err != nil {
				return err
			}
		}

		if err := n.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

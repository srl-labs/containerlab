package core

import (
	"context"
	"errors"

	clablinks "github.com/srl-labs/containerlab/links"
)

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
	if c.LabRuntime != nil {
		return c.startNodesWithLabRuntime(ctx, nodeNames)
	}

	if err := c.ResolveLinks(); err != nil {
		return err
	}

	nodes, err := c.lifecycleNodes(nodeNames)
	if err != nil {
		return err
	}

	var endpoints []clablinks.Endpoint
	for _, n := range nodes {
		if err := n.Start(ctx); err != nil {
			return errors.Join(err, clablinks.ApplyLinkImpairments(ctx, endpoints))
		}

		endpoints = append(endpoints, n.GetEndpoints()...)
	}

	return clablinks.ApplyLinkImpairments(ctx, endpoints)
}

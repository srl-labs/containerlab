package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// lifecycleNodes resolves the requested node names without applying lifecycle policy.
func (c *CLab) lifecycleNodes(nodeNames []string) ([]clabnodes.Node, error) {
	var nodes []clabnodes.Node

	if len(nodeNames) > 0 {
		for _, name := range nodeNames {
			n, ok := c.Nodes[name]
			if !ok {
				// node filter case, where referenced node in filter is not in topo file
				return nil, fmt.Errorf(
					"%w: node %q is not present in the topology",
					claberrors.ErrIncorrectInput,
					name,
				)
			}

			nodes = append(nodes, n)
		}
	} else {
		for _, n := range c.Nodes {
			nodes = append(nodes, n)
		}
	}

	return nodes, nil
}

func (c *CLab) parkRecreatedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.parkedNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		parker, ok := node.(interface{ ParkEndpoints(context.Context) error })
		if !ok {
			return fmt.Errorf("node %q does not support endpoint parking", nodeName)
		}
		log.Info("Parking links for recreate", "node", nodeName)
		if err := parker.ParkEndpoints(ctx); err != nil {
			return fmt.Errorf("failed parking endpoints for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) restoreRecreatedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.parkedNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		restorer, ok := node.(interface{ RestoreEndpoints(context.Context) error })
		if !ok {
			return fmt.Errorf("node %q does not support endpoint restore", nodeName)
		}
		log.Info("Restoring links after recreate", "node", nodeName)
		if err := restorer.RestoreEndpoints(ctx); err != nil {
			return fmt.Errorf("failed restoring endpoints for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) startStoppedNodes(ctx context.Context, plan *applyPlan) error {
	for _, nodeName := range sortedStringSet(plan.startNodeSet) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		log.Info("Starting stopped node", "node", nodeName)
		if err := node.Start(ctx); err != nil {
			return fmt.Errorf("failed starting node %q: %w", nodeName, err)
		}
	}

	return nil
}

func (c *CLab) restartApplyNodes(
	ctx context.Context,
	nodeSet map[string]struct{},
) error {
	nodeNames := sortedStringSet(nodeSet)

	for _, nodeName := range nodeNames {
		node := c.Nodes[nodeName]
		if node == nil {
			continue
		}

		log.Info("Restarting node after link apply", "node", nodeName)
		if err := node.Stop(ctx); err != nil {
			return err
		}
		if err := node.Start(ctx); err != nil {
			return err
		}
		if err := c.waitNodeRunning(ctx, node); err != nil {
			return err
		}
		if err := c.waitNodeHealthyIfAvailable(ctx, node); err != nil {
			return err
		}
	}

	return nil
}

func (c *CLab) waitNodeRunning(ctx context.Context, node clabnodes.Node) error {
	deadline := time.Now().Add(c.timeout)

	for {
		status := node.GetContainerStatus(ctx)
		if clabruntime.ContainerHasJoinableNetns(status) {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf(
				"node %q did not reach running state, current state %s",
				node.GetShortName(),
				status,
			)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (c *CLab) waitNodeHealthyIfAvailable(ctx context.Context, node clabnodes.Node) error {
	if node.Config().Healthcheck == nil {
		return nil
	}

	deadline := time.Now().Add(c.timeout)

	for {
		healthy, err := node.IsHealthy(ctx)
		if healthy {
			return nil
		}
		if err != nil && strings.Contains(err.Error(), "no health information") {
			return nil
		}
		if err != nil {
			log.Debugf("error checking node %q health: %v", node.GetShortName(), err)
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("node %q did not become healthy: %w", node.GetShortName(), err)
			}
			return fmt.Errorf("node %q did not become healthy", node.GetShortName())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

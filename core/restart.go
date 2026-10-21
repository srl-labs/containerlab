package core

import (
	"context"
	"fmt"

	clablinks "github.com/srl-labs/containerlab/links"
)

// RestartNodes performs stop+start, restoring parked interfaces and recreating
// topology links lost when a container exited before its endpoints could be parked.
func (c *CLab) RestartNodes(ctx context.Context, nodeNames []string) error {
	if c.LabRuntime != nil {
		return c.restartNodesWithLabRuntime(ctx, nodeNames)
	}

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

	selected := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		selected[n.GetShortName()] = struct{}{}
	}
	return c.restoreRestartLinks(ctx, selected)
}

// Inspect only links touching restarted nodes. A previously exited container has
// no endpoints to park; Start alone cannot restore those links.
func (c *CLab) restoreRestartLinks(ctx context.Context, selected map[string]struct{}) error {
	plan := newApplyPlan(nil, nil)
	var candidates []clablinks.Link
	for _, link := range c.Links {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			if _, ok := selected[ep.GetNode().GetShortName()]; ok {
				candidates = append(candidates, link)
				break
			}
		}
	}
	for _, link := range candidates {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			node := ep.GetNode()
			name := node.GetShortName()
			if _, seen := plan.endpointNodes[name]; seen {
				continue
			}
			plan.setEndpointNode(name, node)
			interfaces, err := clablinks.DiscoverOwnedInterfacesFor(ctx, node, name,
				trackedRestartEndpointNames(node))
			if err != nil {
				return fmt.Errorf("inspect restart links for node %q: %w", name, err)
			}
			for _, iface := range interfaces {
				key := applyEndpointKey{node: name, iface: iface.Name}
				plan.liveEndpointSet[key] = struct{}{}
				plan.liveEndpointInfo[key] = iface
			}
		}
	}
	var missing []clablinks.Link
	for _, link := range candidates {
		if plan.linkNeedsDeploy(link) {
			missing = append(missing, link)
		}
	}
	if err := c.removeApplyLinkEndpoints(ctx, missing); err != nil {
		return err
	}
	if err := c.deployLinks(ctx, missing, nil); err != nil {
		return err
	}
	// Runtime-created addresses disappear with the lost interfaces too. Replay
	// exec configuration only for nodes whose links were recreated.
	touched := map[string]struct{}{}
	for _, link := range missing {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			name := ep.GetNode().GetShortName()
			if _, exists := c.Nodes[name]; exists {
				touched[name] = struct{}{}
			}
		}
	}
	return c.postDeployApplyNodes(ctx, sortedStringSet(touched), true)
}

func trackedRestartEndpointNames(node clablinks.Node) map[string]struct{} {
	names := map[string]struct{}{}
	for _, ep := range node.GetEndpoints() {
		names[ep.GetIfaceName()] = struct{}{}
	}
	return names
}

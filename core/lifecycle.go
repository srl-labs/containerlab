package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	claberrors "github.com/srl-labs/containerlab/errors"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
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

func (c *CLab) lifecycleStartNodes(
	nodeNames []string,
) ([]clabnodes.Node, map[string]clabtypes.WaitForList, error) {
	selected := map[string]struct{}{}
	deps := map[string]clabtypes.WaitForList{}

	var addWithDependencies func(string) error
	addWithDependencies = func(name string) error {
		n, ok := c.Nodes[name]
		if !ok {
			return fmt.Errorf(
				"%w: node %q is not present in the topology",
				claberrors.ErrIncorrectInput,
				name,
			)
		}

		if _, exists := selected[name]; exists {
			return nil
		}
		selected[name] = struct{}{}

		for _, dep := range c.lifecycleDependencies(n) {
			if _, ok := c.Nodes[dep.Node]; !ok {
				return fmt.Errorf("dependee node %s not found", dep.Node)
			}
			deps[name] = append(deps[name], dep)
			if err := addWithDependencies(dep.Node); err != nil {
				return err
			}
		}

		return nil
	}

	if len(nodeNames) > 0 {
		for _, name := range nodeNames {
			if err := addWithDependencies(name); err != nil {
				return nil, nil, err
			}
		}
	} else {
		names := make([]string, 0, len(c.Nodes))
		for name := range c.Nodes {
			names = append(names, name)
		}
		slices.Sort(names)

		for _, name := range names {
			if err := addWithDependencies(name); err != nil {
				return nil, nil, err
			}
		}
	}

	orderedNames, err := lifecycleToposort(selected, deps)
	if err != nil {
		return nil, nil, err
	}

	nodes := make([]clabnodes.Node, 0, len(orderedNames))
	for _, name := range orderedNames {
		nodes = append(nodes, c.Nodes[name])
	}

	return nodes, deps, nil
}

func (c *CLab) lifecycleDependencies(n clabnodes.Node) clabtypes.WaitForList {
	var deps clabtypes.WaitForList

	if stages := n.Config().Stages; stages != nil {
		for _, waitForNodes := range stages.GetWaitFor() {
			deps = append(deps, waitForNodes...)
		}
	}

	netModeArr := strings.SplitN(n.Config().NetworkMode, ":", 2) //nolint: mnd
	if netModeArr[0] == "container" && len(netModeArr) == 2 {
		if _, exists := c.Nodes[netModeArr[1]]; exists {
			deps = append(deps, &clabtypes.WaitFor{
				Node:  netModeArr[1],
				Stage: clabtypes.WaitForCreate,
			})
		}
	}

	return deps
}

func lifecycleToposort(
	selected map[string]struct{},
	deps map[string]clabtypes.WaitForList,
) ([]string, error) {
	var ordered []string
	visiting := map[string]bool{}
	visited := map[string]bool{}

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("cyclic lifecycle dependencies found at node %q", name)
		}

		visiting[name] = true
		for _, dep := range deps[name] {
			if _, ok := selected[dep.Node]; !ok {
				continue
			}
			if err := visit(dep.Node); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		ordered = append(ordered, name)

		return nil
	}

	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return ordered, nil
}

func (c *CLab) waitForLifecycleNodeHealthy(
	ctx context.Context,
	nodeName string,
	depName string,
) error {
	dep, ok := c.Nodes[depName]
	if !ok {
		return fmt.Errorf("dependee node %s not found", depName)
	}

	for {
		healthy, err := dep.IsHealthy(ctx)
		if err != nil {
			return fmt.Errorf(
				"node %q waiting for node %q healthy stage: %w",
				nodeName,
				depName,
				err,
			)
		}
		if healthy {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

package core

import "fmt"

// networkModeNodeOrder orders selected nodes before their namespace dependents.
// References outside the selection are handled by the caller.
func (c *CLab) networkModeNodeOrder(nodeNames []string) ([]string, error) {
	selected := make(map[string]struct{}, len(nodeNames))
	for _, name := range nodeNames {
		if _, exists := c.Nodes[name]; !exists {
			return nil, fmt.Errorf("node %q not found", name)
		}
		if _, duplicate := selected[name]; duplicate {
			return nil, fmt.Errorf("node %q selected more than once", name)
		}
		selected[name] = struct{}{}
	}
	dependents := make(map[string][]string)
	order := make([]string, 0, len(nodeNames))
	for _, name := range nodeNames {
		target := networkModeContainerTarget(c.Nodes[name].Config().NetworkMode)
		if _, internal := selected[target]; internal {
			dependents[target] = append(dependents[target], name)
		} else {
			order = append(order, name)
		}
	}
	for i := 0; i < len(order); i++ {
		order = append(order, dependents[order[i]]...)
	}
	if len(order) != len(nodeNames) {
		return nil, fmt.Errorf("cyclic network-mode container dependencies")
	}
	return order, nil
}

// planNetworkModeRestarts rebinds dependents after their target's namespace
// changes. Link restarts happen after deployment, so even a freshly deployed
// dependent must restart after a target that is restarted for a link change.
func (c *CLab) planNetworkModeRestarts(plan *applyPlan) error {
	order, err := c.networkModeNodeOrder(sortedNodeNames(c.Nodes))
	if err != nil {
		return err
	}
	for _, name := range order {
		if _, restarting := plan.linkRestartNodeSet[name]; restarting {
			continue
		}
		target := networkModeContainerTarget(c.Nodes[name].Config().NetworkMode)
		_, earlyRestart := plan.restartNodeSet[target]
		_, starting := plan.startNodeSet[target]
		_, lateRestart := plan.linkRestartNodeSet[target]
		if !earlyRestart && !starting && !lateRestart {
			continue
		}
		if !lateRestart {
			if _, starting := plan.startNodeSet[name]; starting {
				continue
			}
			if _, added := plan.addedNodeSet[name]; added {
				continue
			}
			if _, recreated := plan.recreatedNodeSet[name]; recreated {
				continue
			}
		}
		plan.linkRestartNodeSet[name] = struct{}{}
		reason := fmt.Sprintf("network-mode target %q restarted", target)
		if previous := plan.nodeChangeReasons[name]; previous != "" {
			reason = previous + "; " + reason
		}
		plan.nodeChangeReasons[name] = reason
	}
	return nil
}

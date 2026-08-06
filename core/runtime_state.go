package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

type runtimeNodeGroup struct {
	name        string
	containers  []clabruntime.GenericContainer
	distributed bool
	external    bool
}

func (c *CLab) runtimeNodeGroups(
	ctx context.Context,
) (map[string]*runtimeNodeGroup, error) {
	containers, err := c.ListContainers(ctx, WithListLabName(c.Config.Name))
	if err != nil {
		return nil, err
	}

	result := make(map[string]*runtimeNodeGroup)
	var duplicates []string

	for _, ctr := range containers {
		if ctr.Labels[clabconstants.ToolType] != "" {
			continue
		}

		nodeName := ctr.Labels[clabconstants.NodeName]
		if nodeName == "" {
			continue
		}

		groupName := ctr.Labels[clabconstants.RootNodeName]
		distributed := groupName != ""
		if groupName == "" {
			groupName = nodeName
		}

		group, exists := result[groupName]
		if !exists {
			group = &runtimeNodeGroup{
				name:        groupName,
				distributed: distributed,
			}
			result[groupName] = group
		}
		group.containers = append(group.containers, ctr)
		group.distributed = group.distributed || distributed

		if !group.distributed && len(group.containers) > 1 {
			duplicates = append(duplicates, nodeName)
			continue
		}
	}

	// Pre-existing resources have no containerlab labels, but still participate in apply.
	for nodeName, node := range c.Nodes {
		cfg := node.Config()
		if cfg == nil {
			continue
		}
		if _, exists := result[nodeName]; exists {
			continue
		}
		if !cfg.IsRootNamespaceBased &&
			(!cfg.SkipUniquenessCheck || node.GetContainerStatus(ctx) == clabruntime.NotFound) {
			continue
		}
		result[nodeName] = &runtimeNodeGroup{name: nodeName, external: true}
	}

	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf(
			"apply supports only single-container nodes, found multiple containers for nodes: %s",
			strings.Join(duplicates, ", "),
		)
	}

	for _, group := range result {
		sort.Slice(group.containers, func(i, j int) bool {
			return runtimeContainerLess(group.containers[i], group.containers[j])
		})
	}

	return result, nil
}

// NeedsInitialDeploy reports whether the lab has no runtime state yet, i.e. whether
// Deploy would perform a fresh deployment instead of reconciling a running lab.
func (c *CLab) NeedsInitialDeploy(ctx context.Context) (bool, error) {
	currentNodes, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		return false, err
	}

	return c.needsInitialDeploy(currentNodes)
}

func (c *CLab) needsInitialDeploy(currentNodes map[string]*runtimeNodeGroup) (bool, error) {
	if len(currentNodes) == 0 {
		return true, nil
	}
	for _, node := range currentNodes {
		if !node.external {
			return false, nil
		}
	}

	state, err := c.LoadState()
	return state == nil, err
}

func (c *CLab) setMgmtBridgeFromRuntime(
	currentNodes map[string]*runtimeNodeGroup,
) error {
	if c.Config.Mgmt.Bridge != "" {
		return nil
	}

	var bridge string
	for _, runtimeNode := range currentNodes {
		for _, ctr := range runtimeNode.containers {
			ctrBridge := ctr.Labels[clabconstants.NodeMgmtNetBr]
			if ctrBridge == "" {
				continue
			}
			if bridge == "" {
				bridge = ctrBridge
				continue
			}
			if bridge != ctrBridge {
				return fmt.Errorf(
					"runtime lab has conflicting management bridge labels: %q and %q",
					bridge,
					ctrBridge,
				)
			}
		}
	}

	if bridge != "" {
		c.Config.Mgmt.Bridge = bridge
	}

	return nil
}

func (c *CLab) updateRuntimeInfoForExistingNodes(ctx context.Context) error {
	runtimeNodes, err := c.runtimeNodeGroups(ctx)
	if err != nil {
		return err
	}

	for _, nodeName := range sortedRuntimeNodeGroupNames(runtimeNodes) {
		node, exists := c.Nodes[nodeName]
		if !exists {
			continue
		}
		if err := node.UpdateConfigWithRuntimeInfo(ctx); err != nil {
			return fmt.Errorf("failed to update runtime info for node %q: %w", nodeName, err)
		}
	}

	return nil
}

func sortedRuntimeNodeGroupNames(nodes map[string]*runtimeNodeGroup) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func runtimeContainerNodeName(ctr clabruntime.GenericContainer) string {
	if name := ctr.Labels[clabconstants.NodeName]; name != "" {
		return name
	}
	if len(ctr.Names) > 0 {
		return ctr.Names[0]
	}
	return ctr.ID
}

func runtimeContainerLess(a, b clabruntime.GenericContainer) bool {
	aName := runtimeContainerNodeName(a)
	bName := runtimeContainerNodeName(b)

	aOrder := runtimeComponentSortOrder(aName)
	bOrder := runtimeComponentSortOrder(bName)
	if aOrder != bOrder {
		return aOrder < bOrder
	}

	return aName < bName
}

func runtimeComponentSortOrder(name string) int {
	idx := strings.LastIndex(name, "-")
	if idx < 0 || idx == len(name)-1 {
		return 0
	}

	slot := strings.ToUpper(name[idx+1:])
	if len(slot) == 1 && slot[0] >= 'A' && slot[0] <= 'Z' {
		return 100 - int(slot[0])
	}

	var n int
	for _, r := range slot {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}

	return n
}

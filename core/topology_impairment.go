package core

import (
	"fmt"
	"slices"
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
)

const (
	impairmentBridgeExec = `sh -c "until ip link show eth1 >/dev/null 2>&1 && ip link show eth2 >/dev/null 2>&1; do sleep 1; done; ip link add br0 type bridge; ip link set eth1 master br0; ip link set eth2 master br0; ip link set eth1 up; ip link set eth2 up; ip link set br0 up"`
)

func (c *CLab) expandLinkImpairments() error {
	if c.Config.Topology == nil || len(c.Config.Topology.Links) == 0 {
		return nil
	}

	if c.Config.Topology.Nodes == nil {
		c.Config.Topology.Nodes = map[string]*clabtypes.NodeDefinition{}
	}

	expansion, err := clablinks.ExpandImpairments(
		c.Config.Topology.Links,
		c.linkRequiresImpairmentBridge,
	)
	if err != nil {
		return err
	}

	for _, bridgeNode := range expansion.BridgeNodes {
		if _, exists := c.Config.Topology.Nodes[bridgeNode.Name]; exists {
			return fmt.Errorf("link impairment bridge node %q already exists", bridgeNode.Name)
		}

		c.Config.Topology.Nodes[bridgeNode.Name] = newImpairmentBridgeNodeDefinition(bridgeNode)
	}

	c.Config.Topology.Links = expansion.Links

	return nil
}

func (c *CLab) linkRequiresImpairmentBridge(endpointNodes []string) bool {
	if c.Reg == nil {
		return false
	}

	for _, nodeName := range endpointNodes {
		nodeKind := strings.ToLower(c.Config.Topology.GetNodeKind(nodeName))
		kind := c.Reg.Kind(nodeKind)
		if kind != nil && kind.RequiresLinkImpairmentBridge() {
			return true
		}
	}

	return false
}

func newImpairmentBridgeNodeDefinition(bridgeNode *clablinks.ImpairmentBridgeNode) *clabtypes.NodeDefinition {
	return &clabtypes.NodeDefinition{
		Kind:   "linux",
		Image:  bridgeNode.Image,
		CapAdd: []string{"NET_ADMIN"},
		Exec:   []string{impairmentBridgeExec},
		Labels: map[string]string{
			clabconstants.GeneratedNode:           "true",
			clabconstants.GeneratedNodeRole:       "impairment-bridge",
			clabconstants.ImpairmentOriginalNodes: strings.Join(bridgeNode.OriginalNodes, ","),
		},
	}
}

func (c *CLab) expandNodeFilterForGeneratedNodes(nodeFilter []string) []string {
	expandedNodeFilter := append([]string{}, nodeFilter...)
	for nodeName, nodeDef := range c.Config.Topology.Nodes {
		if nodeDef == nil || nodeDef.Labels[clabconstants.GeneratedNodeRole] != "impairment-bridge" {
			continue
		}

		originalNodes := strings.Split(nodeDef.Labels[clabconstants.ImpairmentOriginalNodes], ",")
		if allStringsInSlice(originalNodes, nodeFilter) && !slices.Contains(expandedNodeFilter, nodeName) {
			expandedNodeFilter = append(expandedNodeFilter, nodeName)
		}
	}

	return expandedNodeFilter
}

func allStringsInSlice(values []string, slice []string) bool {
	for _, value := range values {
		if !slices.Contains(slice, value) {
			return false
		}
	}

	return true
}

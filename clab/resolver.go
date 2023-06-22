package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

type DefaultNodeResolver struct {
	c *CLab
}

func NewDefaultNodeResolver(c *CLab) *DefaultNodeResolver {
	return &DefaultNodeResolver{
		c: c,
	}
}

func (r *DefaultNodeResolver) ResolveNode(nodeName string) (types.LinkNode, error) {
	if val, exists := r.c.Nodes[nodeName]; exists {
		return NewDefaultLinkNode(val), nil
	}
	return nil, fmt.Errorf("node %s could not be found", nodeName)
}

type DefaultLinkNode struct {
	node nodes.Node
}

func NewDefaultLinkNode(n nodes.Node) *DefaultLinkNode {
	return &DefaultLinkNode{
		node: n,
	}
}

func (dn *DefaultLinkNode) GetNamespacePath() string {
	return dn.node.Config().NSPath
}

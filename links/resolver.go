package links

import (
	"fmt"

	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/nodes"
)

type DefaultResolver struct {
	c *clab.CLab
}

func NewDefaultResolver(c *clab.CLab) *DefaultResolver {
	return &DefaultResolver{
		c: c,
	}
}

func (r *DefaultResolver) ResolveNode(nodeName string) (nodes.Node, error) {
	if val, exists := r.c.Nodes[nodeName]; exists {
		return val, nil
	}
	return nil, fmt.Errorf("node %s could not be found", nodeName)
}

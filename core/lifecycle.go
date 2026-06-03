package core

import (
	"fmt"

	claberrors "github.com/srl-labs/containerlab/errors"
	clabnodes "github.com/srl-labs/containerlab/nodes"
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

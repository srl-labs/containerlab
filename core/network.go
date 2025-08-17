package core

import (
	"context"

	"github.com/srl-labs/containerlab/labels"
)

func (c *CLab) CreateNetwork(ctx context.Context, caller ...string) error {
	// create docker network or use existing one
	if err := c.globalRuntime().CreateNet(ctx, caller...); err != nil {
		return err
	}

	// save mgmt bridge name as a label
	for _, n := range c.Nodes {
		n.Config().Labels[labels.NodeMgmtNetBr] = c.globalRuntime().Mgmt().Bridge
	}

	return nil
}

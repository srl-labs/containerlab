package clab

import "context"

func (c *CLab) CreateNetwork(ctx context.Context) error {
	// create docker network or use existing one
	if err := c.GlobalRuntime().CreateNet(ctx); err != nil {
		return err
	}

	// save mgmt bridge name as a label
	for _, n := range c.Nodes {
		n.Config().Labels[NodeMgmtNetBr] = c.GlobalRuntime().Mgmt().Bridge
	}

	return nil
}

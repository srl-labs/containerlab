package core

import (
	"context"

	clabconstants "github.com/srl-labs/containerlab/constants"
)

func (c *CLab) CreateNetwork(ctx context.Context) error {
	// create docker network or use existing one
	if err := c.globalRuntime().CreateNet(ctx); err != nil {
		return err
	}

	// save mgmt bridge name as a label
	for _, n := range c.Nodes {
		n.Config().Labels[clabconstants.NodeMgmtNetBr] = c.globalRuntime().Mgmt().Bridge
	}

	return nil
}

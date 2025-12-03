package core

import (
	"context"
	"sync"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
)

func (c *CLab) Save(
	ctx context.Context,
) error {
	err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))

	for _, node := range c.Nodes {
		go func(node clabnodes.Node) {
			defer wg.Done()

			err := node.SaveConfig(ctx)
			if err != nil {
				log.Errorf("err: %v", err)
			}
		}(node)
	}

	wg.Wait()

	return nil
}

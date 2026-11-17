package core

import (
	"context"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

// SetLinkImpairment sets the impairment of a lab link interface through the lab runtime, or
// resets it when the request carries none.
func (c *CLab) SetLinkImpairment(
	ctx context.Context,
	req clablabruntime.ImpairmentRequest,
) error {
	impairer, err := c.linkImpairer()
	if err != nil {
		return err
	}

	if req.Name == "" {
		req.Name = c.Config.Name
	}
	if req.Identity.IsZero() {
		req.Identity = c.labRuntimeIdentity
	}

	return impairer.SetImpairment(ctx, req)
}

// LinkImpairments lists the impaired lab link interfaces of the given nodes, or of every node,
// through the lab runtime.
func (c *CLab) LinkImpairments(
	ctx context.Context,
	nodes []string,
) ([]clablabruntime.NodeImpairments, error) {
	impairer, err := c.linkImpairer()
	if err != nil {
		return nil, err
	}

	return impairer.Impairments(ctx, clablabruntime.NodeRequest{
		Name:     c.Config.Name,
		Nodes:    nodes,
		Identity: c.labRuntimeIdentity,
	})
}

func (c *CLab) linkImpairer() (clablabruntime.LinkImpairer, error) {
	impairer, err := labRuntimeCapability[clablabruntime.LinkImpairer](c, "link impairment")
	if err != nil {
		return nil, err
	}
	if err := c.requireLabName(); err != nil {
		return nil, err
	}

	return impairer, nil
}

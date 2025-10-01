package network

import (
	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

// SendInteractive sends a slice of channel.SendInteractiveEvent to the device. This method wraps
// the generic driver level SendInteractive but handles acquiring the desired privilege level prior
// to returning the generic driver method.
func (d *Driver) SendInteractive(
	events []*channel.SendInteractiveEvent,
	opts ...util.Option,
) (*response.Response, error) {
	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	targetPriv := op.PrivilegeLevel

	if targetPriv == "" {
		targetPriv = d.DefaultDesiredPriv
	}

	err = d.AcquirePriv(targetPriv)
	if err != nil {
		return nil, err
	}

	return d.Driver.SendInteractive(
		events,
		opts...,
	)
}

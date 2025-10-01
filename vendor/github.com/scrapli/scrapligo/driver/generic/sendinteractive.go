package generic

import (
	"strings"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

func joinInputEvents(events []*channel.SendInteractiveEvent) string {
	inputs := make([]string, len(events))

	for i, event := range events {
		inputs[i] = event.ChannelInput
	}

	return strings.Join(inputs, ", ")
}

// SendInteractive sends a slice of channel.SendInteractiveEvent to the device. This method wraps
// the channel level method and returns a response.Response instead of simply bytes.
func (d *Driver) SendInteractive(
	events []*channel.SendInteractiveEvent,
	opts ...util.Option,
) (*response.Response, error) {
	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	if len(op.FailedWhenContains) == 0 {
		op.FailedWhenContains = d.FailedWhenContains
	}

	r := response.NewResponse(
		joinInputEvents(events),
		d.Transport.GetHost(),
		d.Transport.GetPort(),
		op.FailedWhenContains,
	)

	b, err := d.Channel.SendInteractive(
		events,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	r.Record(b)

	return r, nil
}

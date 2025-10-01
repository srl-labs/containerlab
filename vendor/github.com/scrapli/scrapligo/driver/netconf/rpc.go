package netconf

import (
	"context"
	"fmt"
	"time"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

func (d *Driver) buildRPCElem(
	filter string,
) *message {
	netconfInput := d.buildPayload(filter)

	return netconfInput
}

// RPC executes a "bare" RPC against the NETCONF server.
func (d *Driver) RPC(opts ...util.Option) (*response.NetconfResponse, error) {
	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	return d.sendRPC(d.buildRPCElem(op.Filter), op)
}

func (d *Driver) sendRPC(
	m *message,
	op *OperationOptions,
) (*response.NetconfResponse, error) {
	if d.ForceSelfClosingTags {
		d.Logger.Debug("ForceSelfClosingTags is true, enforcing...")
	}

	serialized, err := m.serialize(d.SelectedVersion, d.ForceSelfClosingTags, d.ExcludeHeader)
	if err != nil {
		return nil, err
	}

	d.Logger.Debugf("sending finalized rpc payload:\n%s", string(serialized.framedXML))

	r := response.NewNetconfResponse(
		serialized.rawXML,
		serialized.framedXML,
		d.Transport.GetHost(),
		d.Transport.GetPort(),
		d.SelectedVersion,
	)

	err = d.Channel.WriteAndReturn(serialized.framedXML, false)
	if err != nil {
		return nil, err
	}

	if d.SelectedVersion == V1Dot1 {
		err = d.Channel.WriteReturn()
		if err != nil {
			return nil, err
		}
	}

	done := make(chan []byte)

	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func() {
		defer close(done)

		var data []byte

		for {
			if ctx.Err() != nil {
				// timer expired, we're already done, nobody will be listening for our send anyway
				return
			}

			data = d.getMessage(m.MessageID)
			if data != nil {
				break
			}

			time.Sleep(5 * time.Microsecond) //nolint: mnd
		}

		done <- data
	}()

	timer := time.NewTimer(d.Channel.GetTimeout(op.Timeout))

	select {
	case err = <-d.errs:
		return nil, err
	case <-timer.C:
		d.Logger.Critical("channel timeout sending input to device")

		return nil, fmt.Errorf("%w: channel timeout sending input to device", util.ErrTimeoutError)
	case data := <-done:
		r.Record(data)
	}

	return r, nil
}

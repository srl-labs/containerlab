package generic

import (
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

func (d *Driver) sendCommand(
	command string,
	driverOpts *OperationOptions,
	opts ...util.Option,
) (*response.Response, error) {
	d.Logger.Infof("SendCommand requested, sending '%s'", command)

	if len(driverOpts.FailedWhenContains) == 0 {
		driverOpts.FailedWhenContains = d.FailedWhenContains
	}

	r := response.NewResponse(
		command,
		d.Transport.GetHost(),
		d.Transport.GetPort(),
		driverOpts.FailedWhenContains,
	)

	b, err := d.Channel.SendInput(command, opts...)
	if err != nil {
		return nil, err
	}

	r.Record(b)

	return r, nil
}

// SendCommand sends the input string to the device and returns a response.Response object.
func (d *Driver) SendCommand(command string, opts ...util.Option) (*response.Response, error) {
	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	return d.sendCommand(command, op, opts...)
}

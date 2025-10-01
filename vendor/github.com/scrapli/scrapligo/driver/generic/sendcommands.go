package generic

import (
	"fmt"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

// SendCommands sends the input strings to the device and returns a response.MultiResponse object.
func (d *Driver) SendCommands(
	commands []string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	d.Logger.Infof("SendCommands requested, sending '%s'", commands)

	if len(commands) == 0 {
		return nil, fmt.Errorf("%w: no inputs provided", util.ErrNoOp)
	}

	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	m := response.NewMultiResponse(d.Transport.GetHost())

	for _, input := range commands[:len(commands)-1] {
		var r *response.Response

		r, err = d.sendCommand(
			input,
			op,
			opts...,
		)
		if err != nil {
			return nil, err
		}

		m.AppendResponse(r)

		if op.StopOnFailed && r.Failed != nil {
			d.Logger.Info(
				"encountered failed command, and stop on failed is true," +
					" discontinuing send commands operation",
			)

			return m, err
		}
	}

	r, err := d.sendCommand(
		commands[len(commands)-1],
		op,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	m.AppendResponse(r)

	return m, nil
}

// SendCommandsFromFile sends each line in the provided file f as a command to the device and
// returns a response.MultiResponse object.
func (d *Driver) SendCommandsFromFile(
	f string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	d.Logger.Infof("SendCommandsFromFile requested, loading commands from file '%s'", f)

	inputs, err := util.LoadFileLines(f)
	if err != nil {
		return nil, err
	}

	return d.SendCommands(inputs, opts...)
}

package network

import (
	"fmt"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

// SendCommands sends the command strings to the device and returns a response.MultiResponse object.
// This method will always ensure that the Driver CurrentPriv value is the DefaultDesiredPriv before
// sending any command. If the privilege level is *not* the DefaultDesiredPriv (which is typically
// "privilege-exec" or "exec"), AcquirePriv will be called with a target privilege level of
// DefaultDesiredPriv.
func (d *Driver) SendCommands(
	commands []string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	if d.CurrentPriv != d.DefaultDesiredPriv {
		err := d.AcquirePriv(d.DefaultDesiredPriv)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: failed acquiring default desired privilege level",
				util.ErrPrivilegeError,
			)
		}
	}

	return d.Driver.SendCommands(commands, opts...)
}

// SendCommandsFromFile is a convenience wrapper to send each line of file f as a command via the
// SendCommands method.
func (d *Driver) SendCommandsFromFile(
	f string,
	opts ...util.Option,
) (*response.MultiResponse, error) {
	if d.CurrentPriv != d.DefaultDesiredPriv {
		err := d.AcquirePriv(d.DefaultDesiredPriv)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: failed acquiring default desired privilege level",
				util.ErrPrivilegeError,
			)
		}
	}

	return d.Driver.SendCommandsFromFile(f, opts...)
}

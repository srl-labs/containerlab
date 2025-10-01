package network

import (
	"fmt"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/util"
)

// SendCommand sends the command string to the device and returns a response.Response object. This
// method will always ensure that the Driver CurrentPriv value is the DefaultDesiredPriv before
// sending any command. If the privilege level is *not* the DefaultDesiredPriv (which is typically
// "privilege-exec" or "exec"), AcquirePriv will be called with a target privilege level of
// DefaultDesiredPriv.
func (d *Driver) SendCommand(command string, opts ...util.Option) (*response.Response, error) {
	if d.CurrentPriv != d.DefaultDesiredPriv {
		err := d.AcquirePriv(d.DefaultDesiredPriv)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: failed acquiring default desired privilege level",
				util.ErrPrivilegeError,
			)
		}
	}

	return d.Driver.SendCommand(command, opts...)
}

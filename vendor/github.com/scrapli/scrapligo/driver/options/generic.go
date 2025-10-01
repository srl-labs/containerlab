package options

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/generic"

	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// WithTransportType allows for changing the underlying transport type, valid options are "system",
// (default, /bin/ssh wrapper), "standard" (crypto/ssh), and "telnet".
func WithTransportType(transportType string) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		switch transportType {
		case transport.SystemTransport,
			transport.StandardTransport,
			transport.TelnetTransport,
			transport.FileTransport:
			d.TransportType = transportType
		default:
			return fmt.Errorf(
				"%w: unknown transport type '%s' provided",
				util.ErrBadOption,
				transportType,
			)
		}

		return nil
	}
}

// WithFailedWhenContains allows for setting a slice of strings that if seen in output indicate the
// input has failed.
func WithFailedWhenContains(fw []string) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.FailedWhenContains = fw

		return nil
	}
}

// WithOnOpen allows for setting a function that is called after successfully opening a connection,
// but before returning the connection to the user. Users may use this function to "prepare" a
// device by doing things like disabling paging, or setting some terminal values.
func WithOnOpen(f func(d *generic.Driver) error) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.OnOpen = f

		return nil
	}
}

// WithOnClose allows for setting a function that is called prior to closing the transport. Users
// may wish to execute some command(s) that help gracefully close their connection prior to
// terminating the transport.
func WithOnClose(f func(d *generic.Driver) error) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.OnClose = f

		return nil
	}
}

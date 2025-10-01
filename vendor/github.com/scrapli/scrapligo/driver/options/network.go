package options

import (
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/util"
)

// WithNetworkOnOpen allows for setting a function that is called after successfully opening a
// connection, but before returning the connection to the user. Users may use this function to
// "prepare" a device by doing things like disabling paging, or setting some terminal values. Note
// that this is different from WithOnOpen which operates at the generic.Driver level rather than the
// network.Driver level!
func WithNetworkOnOpen(f func(d *network.Driver) error) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*network.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.OnOpen = f

		return nil
	}
}

// WithNetworkOnClose allows for setting a function that is called prior to closing the transport.
// Users may wish to execute some command(s) that help gracefully close their connection prior to
// terminating the transport. Note that this is different from WithOnClose which operates at the
// generic.Driver level rather than the network.Driver level!
func WithNetworkOnClose(f func(d *network.Driver) error) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*network.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.OnClose = f

		return nil
	}
}

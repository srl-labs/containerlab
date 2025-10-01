package options

import (
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// WithSystemTransportOpenBin sets the binary to be used when opening a System Transport
// connection.
func WithSystemTransportOpenBin(s string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.System)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.OpenBin = s

		return nil
	}
}

// WithSystemTransportOpenArgs sets the ExtraArgs value of the System Transport, these arguments are
// appended to the System transport open command (the command that spawns the connection).
func WithSystemTransportOpenArgs(l []string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.System)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.ExtraArgs = append(t.ExtraArgs, l...)

		return nil
	}
}

// WithSystemTransportOpenArgsOverride sets the arguments used when opening a System Transport
// connection. When explicitly setting arguments in this fashion all ExtraArgs will be ignored.
func WithSystemTransportOpenArgsOverride(l []string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.System)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.OpenArgs = l

		return nil
	}
}

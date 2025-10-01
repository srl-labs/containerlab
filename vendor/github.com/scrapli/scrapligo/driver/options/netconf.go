package options

import (
	"fmt"

	"github.com/scrapli/scrapligo/driver/netconf"

	"github.com/scrapli/scrapligo/util"
)

// WithNetconfPreferredVersion allows for providing a preferred NETCONF version to use with the
// server. The server must advertise the preferred capability or the connection will fail.
func WithNetconfPreferredVersion(s string) util.Option {
	return func(o interface{}) error {
		switch s {
		case netconf.V1Dot0, netconf.V1Dot1:
		default:
			return fmt.Errorf("%w: invalid netconf version '%s' provided", util.ErrBadOption, s)
		}

		d, ok := o.(*netconf.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.PreferredVersion = s

		return nil
	}
}

// WithNetconfForceSelfClosingTags forces the use of "self-closing HTML tags" -- this should only be
// relevant for Juniper devices.
func WithNetconfForceSelfClosingTags() util.Option {
	return func(o interface{}) error {
		d, ok := o.(*netconf.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.ForceSelfClosingTags = true

		return nil
	}
}

// WithNetconfExcludeHeader excludes the XML header from the NETCONF message.
// This is useful for devices that do not support the XML header.
func WithNetconfExcludeHeader() util.Option {
	return func(o interface{}) error {
		d, ok := o.(*netconf.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.ExcludeHeader = true

		return nil
	}
}

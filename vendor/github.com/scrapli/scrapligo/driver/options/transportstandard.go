package options

import (
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// WithStandardTransportExtraCiphers extends the list of ciphers supported by the standard
// transport.
func WithStandardTransportExtraCiphers(l []string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.Standard)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.ExtraCiphers = l

		return nil
	}
}

// WithStandardTransportExtraKexs extends the list of kext (key exchange algorithms) supported by
// the standard transport.
func WithStandardTransportExtraKexs(l []string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.Standard)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.ExtraKexs = l

		return nil
	}
}

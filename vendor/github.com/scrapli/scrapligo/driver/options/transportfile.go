package options

import (
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// WithFileTransportFile sets the file to open for File Transport object, this should only be used
// for testing.
func WithFileTransportFile(s string) util.Option {
	return func(o interface{}) error {
		t, ok := o.(*transport.File)

		if !ok {
			return util.ErrIgnoredOption
		}

		t.F = s

		return nil
	}
}

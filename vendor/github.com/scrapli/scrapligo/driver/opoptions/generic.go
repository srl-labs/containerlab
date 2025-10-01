package opoptions

import (
	"github.com/scrapli/scrapligo/driver/generic"
	"github.com/scrapli/scrapligo/util"
)

// WithStopOnFailed will force any multi input operation to stop at the first sign of "failed"
// output, meaning the first time there is output from the FailedWhenContains slice in the channel
// output.
func WithStopOnFailed() util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.OperationOptions)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.StopOnFailed = true

		return nil
	}
}

// WithFailedWhenContains sets the slice of strings indicating failure for a given operation.
func WithFailedWhenContains(fw []string) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.OperationOptions)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.FailedWhenContains = fw

		return nil
	}
}

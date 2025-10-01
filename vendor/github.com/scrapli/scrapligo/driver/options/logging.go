package options

import (
	"log"

	"github.com/scrapli/scrapligo/driver/generic"

	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/util"
)

// WithLogger accepts a logging.Instance and applies it to the driver object.
func WithLogger(l *logging.Instance) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		d.Logger = l

		return nil
	}
}

// WithDefaultLogger applies the default logging setup to a driver object. This means log.Print
// for the logger function, and "info" for the log level.
func WithDefaultLogger() util.Option {
	return func(o interface{}) error {
		d, ok := o.(*generic.Driver)

		if !ok {
			return util.ErrIgnoredOption
		}

		l, err := logging.NewInstance(
			logging.WithLevel("info"),
			logging.WithLogger(log.Print),
		)
		if err != nil {
			return err
		}

		d.Logger = l

		return nil
	}
}

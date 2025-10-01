package logging

import (
	"fmt"
	"strings"

	"github.com/scrapli/scrapligo/util"
)

// WithLevel sets the logging level for a logging instance.
func WithLevel(s string) util.Option {
	return func(o interface{}) error {
		i, ok := o.(*Instance)

		if !ok {
			return util.ErrIgnoredOption
		}

		s = strings.ToLower(s)

		switch s {
		case Info, Debug, Critical:
			i.Level = s

			return nil
		}

		return fmt.Errorf("%w: log level '%s' is not a valid level", util.ErrBadOption, s)
	}
}

// WithLogger appends a logger (a function that accepts an interface) to the Loggers slice of a
// logging instance.
func WithLogger(f func(...interface{})) util.Option {
	return func(o interface{}) error {
		i, ok := o.(*Instance)

		if ok {
			i.Loggers = append(i.Loggers, f)

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithFormatter sets the Formatter function of the logging instance.
func WithFormatter(f func(string, string) string) util.Option {
	return func(o interface{}) error {
		i, ok := o.(*Instance)

		if ok {
			i.Formatter = f

			return nil
		}

		return util.ErrIgnoredOption
	}
}

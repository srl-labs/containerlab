package opoptions

import (
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/driver/generic"

	"github.com/scrapli/scrapligo/util"
)

// WithCallbackContains sets the "contains" value of a generic.Callback.
func WithCallbackContains(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.Contains = s

		return nil
	}
}

// WithCallbackNotContains sets the "not contains" value of a generic.Callback.
func WithCallbackNotContains(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.NotContains = s

		return nil
	}
}

// WithCallbackContainsRe sets the "contains" regex value of a generic.Callback.
func WithCallbackContainsRe(p *regexp.Regexp) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.ContainsRe = p

		return nil
	}
}

// WithCallbackInsensitive sets the "Insensitive" value of a generic.Callback -- this will force
// read bytes to lower case for comparison with the contains value.
func WithCallbackInsensitive(b bool) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.Insensitive = b

		return nil
	}
}

// WithCallbackResetOutput will cause the generic.Callback to "reset" or zero the read bytes
// after execution and before returning ot the next callback loop.
func WithCallbackResetOutput() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.ResetOutput = true

		return nil
	}
}

// WithCallbackOnce indicates that this callback should only ever execute once.
func WithCallbackOnce() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.Once = true

		return nil
	}
}

// WithCallbackComplete indicates that if encountered, this callback means that the
// SendWithCallbacks operation is complete.
func WithCallbackComplete() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.Complete = true

		return nil
	}
}

// WithCallbackName sets a Name value on the callback, this is purely for human readability.
func WithCallbackName(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.Name = s

		return nil
	}
}

// WithCallbackNextTimeout sets the timeout value to use for the *next* read duration after this
// callback is executed.
func WithCallbackNextTimeout(d time.Duration) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*generic.Callback)

		if !ok {
			return util.ErrIgnoredOption
		}

		c.NextTimeout = d

		return nil
	}
}

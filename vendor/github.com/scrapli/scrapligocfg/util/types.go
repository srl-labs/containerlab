package util

import (
	"errors"
)

// Option is a simple type that accepts an interface and returns an error; this should only be used
// in the context of applying options to an object.
type Option func(o interface{}) error

// NewOperationOptions returns an OperationOptions with all defaults set, and any provided options
// applied.
func NewOperationOptions(opts ...Option) (*OperationOptions, error) {
	o := &OperationOptions{
		Source:              "",
		DiffColorize:        false,
		DiffSideBySideWidth: 0,
		AutoClean:           false,
		Kwargs:              nil,
	}

	for _, option := range opts {
		err := option(o)
		if err != nil {
			if !errors.Is(err, ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return o, nil
}

// OperationOptions is a struct of options for scrapligocfg operations. In order to support possible
// "non-standard" options, the Kwargs field is a simple map[string]string which can allow for
// platforms supporting arbitrary options.
type OperationOptions struct {
	Source              string
	DiffColorize        bool
	DiffSideBySideWidth int
	AutoClean           bool
	Kwargs              map[string]string
}

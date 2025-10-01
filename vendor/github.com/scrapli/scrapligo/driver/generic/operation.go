package generic

import (
	"errors"

	"github.com/scrapli/scrapligo/util"
)

const (
	defaultStopOnFailed = false
)

// OperationOptions is a struct containing "operation" options for the generic driver.
type OperationOptions struct {
	FailedWhenContains []string
	StopOnFailed       bool
}

// NewOperation returns a new OperationOptions object with the defaults set and any provided options
// applied.
func NewOperation(options ...util.Option) (*OperationOptions, error) {
	o := &OperationOptions{
		FailedWhenContains: []string{},
		StopOnFailed:       defaultStopOnFailed,
	}

	for _, option := range options {
		err := option(o)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return o, nil
}

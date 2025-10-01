package channel

import (
	"errors"
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const (
	defaultStripPrompt = true
	defaultEager       = false
	defaultExact       = false
	defaultTimeout     = -1
)

// OperationOptions is a struct containing "operation" options -- things like if the channel should
// strip the device prompt out, send eagerly (without reading until the input), and the timeout for
// the given operation.
type OperationOptions struct {
	StripPrompt           bool
	Eager                 bool
	ExactMatchInput       bool
	Timeout               time.Duration
	CompletePatterns      []*regexp.Regexp
	InterimPromptPatterns []*regexp.Regexp
}

// NewOperation returns a new OperationOptions object with the defaults set and any provided options
// applied.
func NewOperation(options ...util.Option) (*OperationOptions, error) {
	o := &OperationOptions{
		StripPrompt:      defaultStripPrompt,
		Eager:            defaultEager,
		ExactMatchInput:  defaultExact,
		Timeout:          defaultTimeout,
		CompletePatterns: make([]*regexp.Regexp, 0),
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

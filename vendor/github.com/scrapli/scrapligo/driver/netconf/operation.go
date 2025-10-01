package netconf

import (
	"errors"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const (
	defaultFilterType = "subtree"
	defaultTimeout    = -1
)

// OperationOptions is a struct containing "operation" options that are relevant to the Netconf
// Driver.
type OperationOptions struct {
	Filter      string
	FilterType  string
	DefaultType string
	Timeout     time.Duration

	CommitConfirmed          bool
	CommitConfirmTimeout     uint
	CommitConfirmedPersist   string
	CommitConfirmedPersistID string
}

// NewOperation returns a new OperationOptions object with the defaults set and any provided options
// applied.
func NewOperation(options ...util.Option) (*OperationOptions, error) {
	o := &OperationOptions{
		FilterType: defaultFilterType,
		Timeout:    defaultTimeout,
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

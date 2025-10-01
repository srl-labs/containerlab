package opoptions

import (
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/util"
)

// WithFilterType allows for changing of the default filter type (subtree) for NETCONF operations.
func WithFilterType(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.FilterType = s

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithDefaultType allows for changing of the default "default type" type for NETCONF operations.
func WithDefaultType(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.DefaultType = s

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithFilter allows for setting the filter for NETCONF operations.
func WithFilter(s string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.Filter = s

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCommitConfirmed allows setting the confirmed element in a commit operation.
func WithCommitConfirmed() util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.CommitConfirmed = true

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCommitConfirmTimeout allows setting the confirm-timeout element in a commit operation.
func WithCommitConfirmTimeout(t uint) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.CommitConfirmTimeout = t

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCommitConfirmedPersist allows setting the persist element in a commit operation.
func WithCommitConfirmedPersist(label string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.CommitConfirmedPersist = label

			return nil
		}

		return util.ErrIgnoredOption
	}
}

// WithCommitConfirmedPersistID allows setting the persist-id element in a commit operation.
func WithCommitConfirmedPersistID(id string) util.Option {
	return func(o interface{}) error {
		c, ok := o.(*netconf.OperationOptions)

		if ok {
			c.CommitConfirmedPersistID = id

			return nil
		}

		return util.ErrIgnoredOption
	}
}

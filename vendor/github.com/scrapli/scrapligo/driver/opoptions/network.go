package opoptions

import (
	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/util"
)

// WithPrivilegeLevel sets the desired privilege level for the given operation -- typically this
// is used when wanting to send configs at a "non-standard" configuration privilege level, such as
// "configuration-private" or "configuration-exclusive".
func WithPrivilegeLevel(s string) util.Option {
	return func(o interface{}) error {
		d, ok := o.(*network.OperationOptions)

		if ok {
			d.PrivilegeLevel = s

			return nil
		}

		return util.ErrIgnoredOption
	}
}

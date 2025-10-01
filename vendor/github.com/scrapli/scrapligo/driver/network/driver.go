package network

import (
	"errors"
	"fmt"

	"github.com/scrapli/scrapligo/driver/generic"

	"github.com/scrapli/scrapligo/util"
)

// NewDriver returns an instance of Driver for the provided host with the given options set. Any
// options in the driver/options package may be passed to this function -- those options may be
// applied at the network.Driver, generic.Driver, channel.Channel, or Transport depending on the
// specific option.
func NewDriver(
	host string,
	opts ...util.Option,
) (*Driver, error) {
	gd, err := generic.NewDriver(host, opts...)
	if err != nil {
		return nil, err
	}

	d := &Driver{
		Driver:             gd,
		AuthSecondary:      "",
		PrivilegeLevels:    nil,
		DefaultDesiredPriv: "",
		OnOpen:             nil,
		OnClose:            nil,
	}

	for _, option := range opts {
		err = option(d)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	if d.DefaultDesiredPriv == "" || len(d.PrivilegeLevels) == 0 {
		return nil, fmt.Errorf(
			"%w: no default desired privilege level and/or privilege levels provided, "+
				"these are required with 'netework' driver",
			util.ErrBadOption,
		)
	}

	d.UpdatePrivileges()

	return d, nil
}

// Driver embeds generic.Driver and adds "network" centric functionality including privilege level
// understanding and escalation/deescalation.
type Driver struct {
	*generic.Driver

	AuthSecondary string

	PrivilegeLevels    map[string]*PrivilegeLevel
	DefaultDesiredPriv string
	CurrentPriv        string
	privGraph          map[string]map[string]bool

	OnOpen  func(d *Driver) error
	OnClose func(d *Driver) error
}

// Open opens the underlying generic.Driver, and by extension the channel.Channel and Transport
// objects. This should be called prior to executing any SendX methods of the Driver.
func (d *Driver) Open() error {
	err := d.Driver.Open()
	if err != nil {
		return err
	}

	if d.OnOpen != nil {
		err = d.OnOpen(d)
		if err != nil {
			d.Logger.Criticalf("error executing network driver OnOpen, error: %s", err)

			// don't leave the channel (and more importantly, the transport) open if we are going to
			// return an error
			_ = d.Channel.Close()

			return err
		}
	}

	return nil
}

// Close closes the underlying generic.Driver and by extension the channel.Channel and Transport
// objects.
func (d *Driver) Close() error {
	if d.OnClose != nil {
		err := d.OnClose(d)
		if err != nil {
			d.Logger.Criticalf("error running network on close, error: %s", err)
		}
	}

	return d.Driver.Close()
}

package generic

import (
	"errors"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// NewDriver returns an instance of Driver for the provided host with the given options set. Any
// options in the driver/options package may be passed to this function -- those options may be
// applied at the network.Driver, generic.Driver, channel.Channel, or Transport depending on the
// specific option.
func NewDriver(host string, opts ...util.Option) (*Driver, error) {
	d := &Driver{
		Logger:             nil,
		TransportType:      transport.DefaultTransport,
		Transport:          nil,
		Channel:            nil,
		FailedWhenContains: make([]string, 0),
		OnOpen:             nil,
		OnClose:            nil,
	}

	var err error

	for _, option := range opts {
		err = option(d)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	if d.Logger == nil {
		// set a default logging instance w/ no assigned loggers (a noop basically)
		var l *logging.Instance

		l, err = logging.NewInstance()
		if err != nil {
			return nil, err
		}

		d.Logger = l
	}

	d.Transport, err = transport.NewTransport(d.Logger, host, d.TransportType, opts...)
	if err != nil {
		return nil, err
	}

	d.Channel, err = channel.NewChannel(d.Logger, d.Transport, opts...)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// Driver is the primary user interface for scrapligo. All connections to devices revolve around
// this object. The Driver has attributes for a Channel (channel.Channel) and Transport -- these
// objects are responsible for sending and receiving data from the device over the transport layer,
// and the Driver itself provides friendly methods such as SendCommand used to interact with the
// target device.
type Driver struct {
	Logger *logging.Instance

	TransportType string
	Transport     *transport.Transport

	Channel *channel.Channel

	FailedWhenContains []string

	OnOpen  func(d *Driver) error
	OnClose func(d *Driver) error
}

// Open opens the underlying channel.Channel and Transport objects. This should be called prior to
// executing any other Driver methods.
func (d *Driver) Open() error {
	d.Logger.Debugf(
		"opening connection to host '%s' on port '%d'",
		d.Transport.Args.Host,
		d.Transport.Args.Port,
	)

	err := d.Channel.Open()
	if err != nil {
		return err
	}

	if d.OnOpen != nil {
		d.Logger.Debug("generic driver OnOpen set, executing")

		err = d.OnOpen(d)
		if err != nil {
			d.Logger.Criticalf("error executing generic driver OnOpen, error: %s", err)

			// don't leave the channel (and more importantly, the transport) open if we are going to
			// return an error
			_ = d.Channel.Close()

			return err
		}
	}

	d.Logger.Info("connection opened successfully")

	return nil
}

// Close closes the underlying channel.Channel and Transport objects.
func (d *Driver) Close() error {
	d.Logger.Debugf(
		"closing connection to host '%s' on port '%d'",
		d.Transport.Args.Host,
		d.Transport.Args.Port,
	)

	if d.OnClose != nil {
		err := d.OnClose(d)
		if err != nil {
			d.Logger.Criticalf("error executing generic driver OnClose, error: %s", err)
		}
	}

	err := d.Channel.Close()
	if err != nil {
		return err
	}

	d.Logger.Info("connection closed successfully")

	return nil
}

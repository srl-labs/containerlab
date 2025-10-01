package scrapligocfg

import (
	"errors"
	"fmt"

	"github.com/scrapli/scrapligo/logging"

	"github.com/scrapli/scrapligocfg/cisconxos"

	"github.com/scrapli/scrapligocfg/juniperjunos"

	"github.com/scrapli/scrapligocfg/ciscoiosxr"

	"github.com/scrapli/scrapligocfg/aristaeos"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligocfg/ciscoiosxe"
	"github.com/scrapli/scrapligocfg/util"
)

const (
	// CiscoIOSXE is the Cisco IOS-XE scrapligocfg platform type.
	CiscoIOSXE = "cisco_iosxe"
	// CiscoNXOS is the Cisco NX-OS scrapligocfg platform type.
	CiscoNXOS = "cisco_nxos"
	// CiscoIOSXR is the Cisco IOS-XR scrapligocfg platform type.
	CiscoIOSXR = "cisco_iosxr"
	// AristaEOS is the Arista EOS scrapligocfg platform type.
	AristaEOS = "arista_eos"
	// JuniperJUNOS is the Juniper JunOS scrapligocfg platform type.
	JuniperJUNOS = "juniper_junos"
)

// SupportedPlatforms returns a slice of all available supported "cfg" platforms.
func SupportedPlatforms() []string {
	return []string{
		CiscoIOSXE,
		CiscoNXOS,
		CiscoIOSXR,
		AristaEOS,
		JuniperJUNOS,
	}
}

// NewCfg returns an instance of Cfg for the provided platform type.
func NewCfg(conn *network.Driver, platform string, opts ...util.Option) (*Cfg, error) {
	var impl Platform

	var err error

	switch platform {
	case CiscoIOSXE:
		impl, err = ciscoiosxe.NewCiscoIOSXE(conn, opts...)
	case CiscoNXOS:
		impl, err = cisconxos.NewCiscoNXOS(conn, opts...)
	case CiscoIOSXR:
		impl, err = ciscoiosxr.NewCiscoIOSXR(conn, opts...)
	case AristaEOS:
		impl, err = aristaeos.NewAristaEOS(conn, opts...)
	case JuniperJUNOS:
		impl, err = juniperjunos.NewJuniperJunOS(conn, opts...)
	default:
		return nil, fmt.Errorf("%w: ", util.ErrUnsupportedPlatform)
	}

	if err != nil {
		return nil, err
	}

	c := &Cfg{
		Logger: nil,
		Impl:   impl,
		Conn:   conn,
	}

	for _, option := range opts {
		err = option(c)

		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	if c.Logger == nil {
		// set a default logging instance w/ no assigned loggers (a noop basically)
		var l *logging.Instance

		l, err = logging.NewInstance()
		if err != nil {
			return nil, err
		}

		c.Logger = l
	}

	return c, nil
}

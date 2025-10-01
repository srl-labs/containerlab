package transport

import (
	"errors"
	"sync"

	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/util"
)

// GetTransportNames is returns a slice of available transport type names.
func GetTransportNames() []string {
	return []string{SystemTransport, StandardTransport, TelnetTransport}
}

// GetNetconfTransportNames returns a slice of available NETCONF transport type names.
func GetNetconfTransportNames() []string {
	return []string{SystemTransport, StandardTransport}
}

// NewTransport returns an instance of Transport with the requested transport implementation (as
// defined in transportType) set. Typically, users should not need to call this as the process of
// Driver creation will handle this for you.
func NewTransport(
	l *logging.Instance,
	host, transportType string,
	options ...util.Option,
) (*Transport, error) {
	var i Implementation

	var err error

	var args *Args

	args, err = NewArgs(l, host, options...)
	if err != nil {
		return nil, err
	}

	if args.UserImplementation != nil {
		i = args.UserImplementation
	} else {
		switch transportType {
		case SystemTransport, StandardTransport:
			var sshArgs *SSHArgs

			sshArgs, err = NewSSHArgs(options...)
			if err != nil {
				return nil, err
			}

			switch transportType {
			case SystemTransport:
				i, err = NewSystemTransport(sshArgs)
			case StandardTransport:
				i, err = NewStandardTransport(sshArgs)
			}
		case TelnetTransport:
			var telnetArgs *TelnetArgs

			telnetArgs, err = NewTelnetArgs(options...)
			if err != nil {
				return nil, err
			}

			i, err = NewTelnetTransport(telnetArgs)
		case FileTransport:
			i, err = NewFileTransport()
		}

		if err != nil {
			return nil, err
		}
	}

	for _, option := range options {
		err = option(i)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	t := &Transport{
		Args:        args,
		Impl:        i,
		implLock:    &sync.Mutex{},
		timeoutLock: &sync.Mutex{},
	}

	return t, nil
}

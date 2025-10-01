package transport

import (
	"fmt"
	"net"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const (
	// TelnetTransport is the telnet transport for scrapligo.
	TelnetTransport = "telnet"

	iac  = byte(255)
	dont = byte(254)
	do   = byte(253)
	wont = byte(252)
	will = byte(251)
	sga  = byte(3)

	controlCharSocketTimeoutDivisor = 4
)

// NewTelnetTransport returns an instance of Telnet transport.
func NewTelnetTransport(a *TelnetArgs) (*Telnet, error) {
	t := &Telnet{
		TelnetArgs: a,
	}

	return t, nil
}

// Telnet is the telnet transport object.
type Telnet struct {
	TelnetArgs *TelnetArgs
	c          net.Conn
	initialBuf []byte
}

func (t *Telnet) handleControlCharResponse(ctrlBuf []byte, c byte) ([]byte, error) {
	if len(ctrlBuf) == 0 { //nolint:nestif,gocritic
		if c != iac {
			t.initialBuf = append(t.initialBuf, c)
		} else {
			ctrlBuf = append(ctrlBuf, c)
		}
	} else if len(ctrlBuf) == 1 && util.ByteIsAny(c, []byte{do, dont, will, wont}) {
		ctrlBuf = append(ctrlBuf, c)
	} else if len(ctrlBuf) == 2 { //nolint:mnd
		cmd := ctrlBuf[1:2][0]
		ctrlBuf = make([]byte, 0)

		var writeErr error

		if cmd == do && c == sga { //nolint: gocritic
			_, writeErr = t.c.Write([]byte{iac, will, c})
		} else if util.ByteIsAny(cmd, []byte{do, dont}) {
			_, writeErr = t.c.Write([]byte{iac, wont, c})
		} else if cmd == will {
			_, writeErr = t.c.Write([]byte{iac, do, c})
		} else if cmd == wont {
			_, writeErr = t.c.Write([]byte{iac, dont, c})
		}

		if writeErr != nil {
			return nil, writeErr
		}
	}

	return ctrlBuf, nil
}

func (t *Telnet) handleControlChars(a *Args) error {
	d := a.TimeoutSocket / controlCharSocketTimeoutDivisor

	var handleErr error

	ctrlBuf := make([]byte, 0)

	for {
		setDeadlineErr := t.c.SetReadDeadline(time.Now().Add(d))
		if setDeadlineErr != nil {
			return setDeadlineErr
		}

		// speed up timeout after initial Read
		d = a.TimeoutSocket / controlCharSocketTimeoutDivisor * 2 //nolint:mnd

		charBuf := make([]byte, 1)

		_, err := t.c.Read(charBuf)
		if err != nil { //nolint:nestif
			if opErr, ok := err.(*net.OpError); ok {
				if opErr.Timeout() {
					// timeout is good -- we want to be done reading control chars, so cancel the
					// deadline by setting it to "zero"
					cancelDeadlineErr := t.c.SetReadDeadline(time.Time{})
					if cancelDeadlineErr != nil {
						return cancelDeadlineErr
					}

					return nil
				}

				return opErr
			}

			return err
		}

		ctrlBuf, handleErr = t.handleControlCharResponse(ctrlBuf, charBuf[0])
		if handleErr != nil {
			return handleErr
		}
	}
}

// Open opens the Telnet connection.
func (t *Telnet) Open(a *Args) error {
	var err error

	t.c, err = net.Dial(tcp, fmt.Sprintf("%s:%d", a.Host, a.Port))
	if err != nil {
		return err
	}

	err = t.handleControlChars(a)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the Telnet connection.
func (t *Telnet) Close() error {
	return t.c.Close()
}

// IsAlive returns true if the connection (c) attribute of the Telnet object is not nil.
func (t *Telnet) IsAlive() bool {
	return t.c != nil
}

// Read reads n bytes from the transport.
func (t *Telnet) Read(n int) ([]byte, error) {
	if len(t.initialBuf) > 0 {
		b := t.initialBuf
		t.initialBuf = nil

		return b, nil
	}

	b := make([]byte, n)

	n, err := t.c.Read(b)

	return b[0:n], err
}

// Write writes bytes b to the transport.
func (t *Telnet) Write(b []byte) error {
	_, err := t.c.Write(b)

	return err
}

// GetInChannelAuthType returns the in channel auth flavor for the telnet transport.
func (t *Telnet) GetInChannelAuthType() InChannelAuthType {
	return InChannelAuthTelnet
}

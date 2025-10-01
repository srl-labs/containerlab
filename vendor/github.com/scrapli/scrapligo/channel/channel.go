package channel

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

const (
	// DefaultTimeoutOpsSeconds is the default time value for operations -- 60 seconds.
	DefaultTimeoutOpsSeconds = 60
	// DefaultReadDelayMicroSeconds is the default value for the delay between reads of the
	// transport -- 250 microseconds. Going very low is likely to lead to very high cpu and not
	// yield any recognizable gains, so be careful changing this!
	DefaultReadDelayMicroSeconds = 250
	// DefaultReturnChar is the character used to send an "enter" key to the device, "\n".
	DefaultReturnChar = "\n"
	// DefaultPromptSearchDepth -- is the default depth to search for the prompt in the received
	// bytes.
	DefaultPromptSearchDepth = 1_000

	redacted         = "redacted"
	readDelayDivisor = 1_000
)

var (
	promptPattern     *regexp.Regexp //nolint:gochecknoglobals
	promptPatternOnce sync.Once      //nolint:gochecknoglobals
)

func getPromptPattern() *regexp.Regexp {
	promptPatternOnce.Do(func() {
		promptPattern = regexp.MustCompile(`(?im)^[a-z\d.\-@()/:]{1,48}[#>$]\s*$`)
	})

	return promptPattern
}

// NewChannel returns a scrapligo Channel object.
func NewChannel(
	l *logging.Instance,
	t *transport.Transport,
	options ...util.Option,
) (*Channel, error) {
	patterns := getAuthPatterns()

	c := &Channel{
		l: l,
		t: t,

		TimeoutOps: DefaultTimeoutOpsSeconds * time.Second,
		ReadDelay:  DefaultReadDelayMicroSeconds * time.Microsecond,

		UsernamePattern:   patterns.username,
		PasswordPattern:   patterns.password,
		PassphrasePattern: patterns.passphrase,

		PromptSearchDepth: DefaultPromptSearchDepth,
		PromptPattern:     getPromptPattern(),
		ReturnChar:        []byte(DefaultReturnChar),

		done: make(chan struct{}),

		Q:    util.NewQueue(),
		Errs: make(chan error),

		ChannelLog: nil,
	}

	for _, option := range options {
		err := option(c)
		if err != nil {
			if !errors.Is(err, util.ErrIgnoredOption) {
				return nil, err
			}
		}
	}

	return c, nil
}

// Channel is an object that sits "on top" of a scrapligo Transport object, its purpose in life is
// to read data from the transport into its Q, and provide methods to read "until" an input or an
// expected prompt is seen.
type Channel struct {
	l *logging.Instance
	t *transport.Transport

	TimeoutOps time.Duration
	ReadDelay  time.Duration

	AuthBypass bool

	UsernamePattern   *regexp.Regexp
	PasswordPattern   *regexp.Regexp
	PassphrasePattern *regexp.Regexp

	PromptSearchDepth int
	PromptPattern     *regexp.Regexp
	ReturnChar        []byte

	done chan struct{}

	Q              *util.Queue
	Errs           chan error
	readLoopExited bool

	ChannelLog io.Writer
}

// Open opens the underlying Transport and begins the `read` goroutine, this also kicks off any
// in channel authentication (if necessary).
func (c *Channel) Open() (reterr error) {
	err := c.t.Open()
	if err != nil {
		c.l.Criticalf("error opening channel, error: %s", err)

		return err
	}

	defer func() {
		if reterr != nil {
			// don't leave the transport open if we are going to return an error -- especially
			// important for system transport which may leave ptys hanging open if not closed
			// nicely, see #135.
			_ = c.Close()
		}
	}()

	c.l.Debug("starting channel read loop")

	go c.read()

	if c.AuthBypass {
		c.l.Debug("auth bypass is enabled, skipping in channel auth check")

		return nil
	}

	var b []byte

	authData := c.t.InChannelAuthData()

	switch authData.Type {
	case transport.InChannelAuthSSH:
		c.l.Debug("transport requests in channel ssh auth, starting...")

		b, err = c.AuthenticateSSH(
			[]byte(authData.Password),
			[]byte(authData.PrivateKeyPassPhrase),
		)
		if err != nil {
			return err
		}
	case transport.InChannelAuthTelnet:
		c.l.Debug("transport requests in channel telnet auth, starting...")

		b, err = c.AuthenticateTelnet([]byte(authData.User), []byte(authData.Password))
		if err != nil {
			return err
		}
	case transport.InChannelAuthUnsupported:
	}

	if len(b) > 0 {
		// requeue any buffer data we get during in channel authentication back onto the
		// read buffer. mostly this should only be relevant for netconf where we need to
		// read the server capabilities.
		c.Q.Requeue(b)
	}

	return nil
}

// Close signals to stop the channel read loop and closes the underlying Transport object.
func (c *Channel) Close() error {
	c.l.Info("channel closing...")

	close(c.Errs)

	ch := make(chan struct{})

	if !c.readLoopExited {
		go func() {
			defer close(ch)

			c.done <- struct{}{}
		}()
	} else {
		close(ch)
	}

	select {
	case <-ch:
		c.l.Debug("closing underlying transport...")

		return c.t.Close(false)
	case <-time.After(c.ReadDelay * (c.ReadDelay / readDelayDivisor)): //nolint:durationcheck
		// channel is stuck in a blocking read (almost always the case for netconf!), force close
		// transport to finish closing connection, so give it c.ReadDelay*(c.ReadDelay/1000) to
		// "nicely" exit -- with defaults this ends up being 62.5ms.
		c.l.Debug("force closing underlying transport...")

		return c.t.Close(true)
	}
}

type result struct {
	b   []byte
	err error
}

func (c *Channel) processOut(b []byte, strip bool) []byte {
	lines := bytes.Split(b, []byte("\n"))

	cleanLines := make([][]byte, len(lines))
	for i, l := range lines {
		cleanLines[i] = bytes.TrimRight(l, " ")
	}

	b = bytes.Join(cleanLines, []byte("\n"))

	if strip {
		b = c.PromptPattern.ReplaceAll(b, nil)
	}

	b = bytes.Trim(b, string(c.ReturnChar))
	b = bytes.Trim(b, "\n")

	return b
}

// GetTimeout returns the target timeout for an operation based on the TimeoutOps attribute of the
// Channel and the value t.
func (c *Channel) GetTimeout(t time.Duration) time.Duration {
	if t == -1 {
		return c.TimeoutOps
	}

	if t == 0 {
		return util.MaxTimeout * time.Second
	}

	return t
}

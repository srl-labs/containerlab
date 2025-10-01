package generic

import (
	"bytes"
	"fmt"
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/response"

	"github.com/scrapli/scrapligo/util"
)

// NewCallback returns a Callback object with provided options applied.
func NewCallback(
	callback func(*Driver, string) error,
	opts ...util.Option,
) (*Callback, error) {
	c := &Callback{
		Callback:      callback,
		Contains:      "",
		containsBytes: nil,
		ContainsRe:    nil,
		Insensitive:   true,
		ResetOutput:   true,
		Once:          false,
		NextTimeout:   0,
		triggered:     false,
		Complete:      false,
		Name:          "",
	}

	for _, option := range opts {
		err := option(c)
		if err != nil {
			return nil, err
		}
	}

	if c.Contains == "" && c.ContainsRe == nil {
		return nil, fmt.Errorf("%w: must provide contains or contains regex", util.ErrBadOption)
	}

	return c, nil
}

// Callback represents not only a callback function, but what triggers that callback function. This
// object is only used in conjunction with the SendWithCallbacks method.
type Callback struct {
	Callback         func(*Driver, string) error
	Contains         string
	containsBytes    []byte
	NotContains      string
	notContainsBytes []byte
	ContainsRe       *regexp.Regexp
	Insensitive      bool
	// ResetOutput bool indicating if the output should be reset or not after callback execution.
	ResetOutput bool
	// Once bool indicating if this callback should be executed only one time.
	Once bool
	// NextTimout timeout value to use for the subsequent read loop - ignored if Complete is true.
	NextTimeout time.Duration
	triggered   bool
	Complete    bool
	Name        string
}

func (c *Callback) contains() []byte {
	if len(c.containsBytes) == 0 {
		c.containsBytes = []byte(c.Contains)

		if c.Insensitive {
			c.containsBytes = bytes.ToLower(c.containsBytes)
		}
	}

	return c.containsBytes
}

func (c *Callback) notContains() []byte {
	if len(c.notContainsBytes) == 0 {
		c.notContainsBytes = []byte(c.NotContains)

		if c.Insensitive {
			c.notContainsBytes = bytes.ToLower(c.notContainsBytes)
		}
	}

	return c.notContainsBytes
}

func (c *Callback) check(b []byte) bool {
	if c.Insensitive {
		b = bytes.ToLower(b)
	}

	if (c.Contains != "" && bytes.Contains(b, c.contains())) &&
		!(c.NotContains != "" && !bytes.Contains(b, c.notContains())) {
		return true
	}

	if (c.ContainsRe != nil && c.ContainsRe.Match(b)) &&
		!(c.NotContains != "" && !bytes.Contains(b, c.notContains())) {
		return true
	}

	return false
}

type callbackResult struct {
	i         int
	callbacks []*Callback
	b         []byte
	fb        []byte
	err       error
}

func (d *Driver) executeCallback(
	i int,
	callbacks []*Callback,
	b, fb []byte,
	t time.Duration,
) ([]byte, error) {
	cb := callbacks[i]

	if cb.Once {
		if cb.triggered {
			return nil, fmt.Errorf(
				"%w: callback once set, and callback already triggered",
				util.ErrOperationError,
			)
		}

		cb.triggered = true
	}

	if cb.Callback != nil {
		// you might not want to set a callback on the "done" stage, so we skip executing if
		// callback is nil
		err := cb.Callback(d, string(b))
		if err != nil {
			return nil, err
		}
	}

	if cb.Complete {
		return fb, nil
	}

	if cb.ResetOutput {
		b = nil
	}

	nt := t
	if cb.NextTimeout != 0 {
		nt = cb.NextTimeout
	}

	return d.handleCallbacks(callbacks, b, fb, nt)
}

func (d *Driver) handleCallbacks(
	callbacks []*Callback,
	b, fb []byte,
	timeout time.Duration,
) ([]byte, error) {
	c := make(chan *callbackResult)

	go func() {
		defer close(c)

		for {
			rb, err := d.Channel.Read()
			if err != nil {
				c <- &callbackResult{
					err: err,
				}

				return
			}

			b = append(b, rb...)
			fb = append(fb, rb...)

			for i, cb := range callbacks {
				if cb.check(b) {
					c <- &callbackResult{
						i:         i,
						callbacks: callbacks,
						b:         b,
						fb:        fb,
						err:       nil,
					}

					return
				}
			}
		}
	}()

	timer := time.NewTimer(timeout)

	select {
	case r := <-c:
		if r.err != nil {
			return nil, r.err
		}

		return d.executeCallback(r.i, r.callbacks, r.b, r.fb, timeout)
	case <-timer.C:
		return nil, fmt.Errorf("%w: timeout handling callbacks", util.ErrTimeoutError)
	}
}

// SendWithCallbacks sends some input and responds to the output of that input based on the list of
// callbacks provided. This method can be looked at as a more advanced SendInteractive.
func (d *Driver) SendWithCallbacks(
	input string,
	callbacks []*Callback,
	timeout time.Duration,
	opts ...util.Option,
) (*response.Response, error) {
	d.Logger.Info("SendWithCallbacks requested")

	driverOpts, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	if len(driverOpts.FailedWhenContains) == 0 {
		driverOpts.FailedWhenContains = d.FailedWhenContains
	}

	r := response.NewResponse(
		input,
		d.Transport.GetHost(),
		d.Transport.GetPort(),
		driverOpts.FailedWhenContains,
	)

	if input != "" {
		err := d.Channel.WriteAndReturn([]byte(input), false)
		if err != nil {
			return nil, err
		}
	}

	b, err := d.handleCallbacks(callbacks, nil, nil, timeout)
	if err != nil {
		return nil, err
	}

	r.Record(b)

	return r, nil
}

package channel

import (
	"context"
	"errors"
	"fmt"

	"github.com/scrapli/scrapligo/util"
)

// GetPrompt returns a byte slice containing the current "prompt" of the connected ssh/telnet
// server.
func (c *Channel) GetPrompt() ([]byte, error) {
	c.l.Debug("channel GetPrompt requested")

	cr := make(chan *result)

	ctx, cancel := context.WithTimeout(context.Background(), c.TimeoutOps)

	defer cancel()

	go func() {
		defer close(cr)

		err := c.WriteReturn()
		if err != nil {
			cr <- &result{b: nil, err: err}

			return
		}

		var b []byte

		b, err = c.ReadUntilPrompt(ctx)

		// we already know the pattern is in the buf, we just want ot re to yoink it out without
		// any newlines or extra stuff we read (which shouldn't happen outside the initial
		// connection but...)
		cr <- &result{b: c.PromptPattern.Find(b), err: err}
	}()

	r := <-cr
	if r.err != nil {
		if errors.Is(r.err, context.DeadlineExceeded) {
			c.l.Critical("channel timeout fetching prompt")

			return nil, fmt.Errorf(
				"%w: channel timeout fetching prompt",
				util.ErrTimeoutError,
			)
		}

		return nil, r.err
	}

	return r.b, nil
}

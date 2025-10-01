package channel

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/scrapli/scrapligo/util"
)

// SendInputB sends the given input bytes to the device and returns the bytes read.
func (c *Channel) SendInputB(input []byte, opts ...util.Option) ([]byte, error) {
	c.l.Debugf("channel SendInput requested, sending input '%s'", input)

	op, err := NewOperation(opts...)
	if err != nil {
		return nil, err
	}

	readUntilF := c.ReadUntilFuzzy

	if op.ExactMatchInput {
		readUntilF = c.ReadUntilExplicit
	}

	cr := make(chan *result)

	ctx, cancel := context.WithTimeout(context.Background(), c.GetTimeout(op.Timeout))

	// we'll call cancel no matter what, either the read goroutines finished nicely in which case it
	// doesnt matter, or we hit the timer and the cancel will stop the reading
	defer cancel()

	go func() {
		var b []byte

		err = c.Write(input, false)
		if err != nil {
			cr <- &result{b: b, err: err}

			return
		}

		_, err = readUntilF(input)
		if err != nil {
			cr <- &result{b: b, err: err}

			return
		}

		err = c.WriteReturn()
		if err != nil {
			cr <- &result{b: b, err: err}

			return
		}

		if !op.Eager {
			var nb []byte

			var readErr error

			if len(op.InterimPromptPatterns) == 0 {
				nb, readErr = c.ReadUntilPrompt(ctx)
			} else {
				prompts := []*regexp.Regexp{c.PromptPattern}
				prompts = append(prompts, op.InterimPromptPatterns...)

				nb, readErr = c.ReadUntilAnyPrompt(ctx, prompts)
			}

			if readErr != nil {
				cr <- &result{b: b, err: readErr}

				return
			}

			b = append(b, nb...)
		}

		cr <- &result{
			b:   c.processOut(b, op.StripPrompt),
			err: nil,
		}
	}()

	r := <-cr
	if r.err != nil {
		if errors.Is(r.err, context.DeadlineExceeded) {
			c.l.Critical("channel timeout sending input to device")

			return nil, fmt.Errorf(
				"%w: channel timeout sending input to device",
				util.ErrTimeoutError,
			)
		}

		return nil, r.err
	}

	return r.b, nil
}

// SendInput sends the input string to the target device. Any bytes output is returned.
func (c *Channel) SendInput(input string, opts ...util.Option) ([]byte, error) {
	return c.SendInputB([]byte(input), opts...)
}

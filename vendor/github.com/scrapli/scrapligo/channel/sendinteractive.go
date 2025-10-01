package channel

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/scrapli/scrapligo/util"
)

// SendInteractiveEvent is a struct representing a single "event" that can be sent to
// SendInteractive this event contains the input to send, the response to expect, and whether
// scrapligo should expect to see the input or if it is hidden (as is the case with passwords for
// privilege escalation).
type SendInteractiveEvent struct {
	ChannelInput    string
	ChannelResponse string
	HideInput       bool
}

func (c *Channel) sendInteractive(
	ctx context.Context,
	cr chan *result,
	events []*SendInteractiveEvent,
	op *OperationOptions,
	readUntilF func(b []byte) ([]byte, error),
) {
	defer close(cr)

	var b []byte

	for i, e := range events {
		prompts := op.CompletePatterns
		if e.ChannelResponse != "" {
			prompts = append(prompts, regexp.MustCompile(e.ChannelResponse))
		} else {
			prompts = append(prompts, c.PromptPattern)
		}

		err := c.Write([]byte(e.ChannelInput), e.HideInput)
		if err != nil {
			cr <- &result{b: nil, err: err}

			return
		}

		if e.ChannelResponse != "" && !e.HideInput {
			var nb []byte

			nb, err = readUntilF([]byte(e.ChannelInput))
			if err != nil {
				cr <- &result{b: nil, err: err}

				return
			}

			b = append(b, nb...)
		}

		err = c.WriteReturn()
		if err != nil {
			cr <- &result{b: nil, err: err}

			return
		}

		var pb []byte

		pb, err = c.ReadUntilAnyPrompt(ctx, prompts)
		if err != nil {
			cr <- &result{b: nil, err: err}

			return
		}

		b = append(b, pb...)

		if i < len(events)-1 && len(op.CompletePatterns) > 0 {
			var done bool

			for _, p := range op.CompletePatterns {
				if p.Match(pb) {
					done = true

					break
				}
			}

			if done {
				break
			}
		}
	}

	cr <- &result{b: c.processOut(b, false), err: nil}
}

// SendInteractive sends a slice of SendInteractiveEvent to the device. This is typically used to
// handle any well understood "interactive" prompts on a device -- things like "clear logging" which
// prompts the user to confirm, or handling privilege escalation where there is a password prompt.
func (c *Channel) SendInteractive(
	events []*SendInteractiveEvent,
	opts ...util.Option,
) ([]byte, error) {
	c.l.Debugf("channel SendInteractive requested, processing events '%v'", events)

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

	defer cancel()

	go c.sendInteractive(ctx, cr, events, op, readUntilF)

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

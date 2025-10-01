package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"time"

	"github.com/scrapli/scrapligo/util"
)

const inputSearchDepthMultiplier = 2

func getProcessReadBufSearchDepth(promptSearchDepth, inputLen int) int {
	finalSearchDepth := promptSearchDepth

	possibleSearchDepth := inputSearchDepthMultiplier * inputLen

	if possibleSearchDepth > finalSearchDepth {
		finalSearchDepth = possibleSearchDepth
	}

	return finalSearchDepth
}

func processReadBuf(rb []byte, searchDepth int) []byte {
	if len(rb) <= searchDepth {
		return rb
	}

	prb := rb[len(rb)-searchDepth:]

	partitionIdx := bytes.Index(prb, []byte("\n"))

	if partitionIdx > 0 {
		prb = prb[partitionIdx:]
	}

	return prb
}

func (c *Channel) read() {
	defer func() {
		c.readLoopExited = true
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		b, err := c.t.Read()
		if err != nil {
			select {
			case <-c.done:
				// this prevents us from ever writing to, what would in this case be, a closed
				// errs channel. also if we are "done" we probably only got an error about transport
				// dying so we can safely ignore that
				return
			default:
			}

			if errors.Is(err, io.EOF) {
				// the underlying transport was closed so just return, we *probably* will have
				// already bailed out by reading from the (maybe/probably) closed done channel, but
				// if we hit EOF we know we are done anyway
				return
			}

			// we got a transport error, put it into the error channel for processing during
			// the next read activity, log it, sleep and then try again...
			c.l.Criticalf(
				"encountered error reading from transport during channel read loop. error: %s", err,
			)

			c.Errs <- err

			time.Sleep(c.ReadDelay)

			continue
		}

		if len(b) == 0 {
			// nothing to process... no reason to enqueue empty bytes, sleep and then continue...
			time.Sleep(c.ReadDelay)

			continue
		}

		// not 100% this is required, but has existed in scrapli/scrapligo for a long time and am
		// afraid to remove it!
		b = bytes.ReplaceAll(b, []byte("\r"), []byte(""))

		if bytes.Contains(b, []byte("\x1b")) {
			b = util.StripANSI(b)
		}

		c.Q.Enqueue(b)

		if c.ChannelLog != nil {
			_, err = c.ChannelLog.Write(b)
			if err != nil {
				c.l.Criticalf("error writing to channel log, ignoring. error: %s", err)
			}
		}

		time.Sleep(c.ReadDelay)
	}
}

// Read reads and returns the first available bytes from the channel Q object. If there are any
// errors on the Errs channel (these would come from the underlying transport), the error is
// returned with nil for the byte slice.
func (c *Channel) Read() ([]byte, error) {
	select {
	case err := <-c.Errs:
		return nil, err
	default:
	}

	if c.readLoopExited {
		return nil, util.ErrConnectionError
	}

	b := c.Q.Dequeue()

	if b == nil {
		return nil, nil
	}

	c.l.Debugf("channel read %#v", string(b))

	return b, nil
}

// ReadAll reads and returns *all* available bytes form the channel Q object. If there are any
// errors on the Errs channel  (these would come from the underlying transport), the error is
// returned with nil for the byte slice. Be careful using this as it is possible to dequeue "too
// much" from the channel causing us to not be able to "find" the prompt or inputs during normal
// operations. In general, this should probably only be used when connecting to consoles/files.
func (c *Channel) ReadAll() ([]byte, error) {
	select {
	case err := <-c.Errs:
		return nil, err
	default:
	}

	b := c.Q.DequeueAll()

	if b == nil {
		return nil, nil
	}

	c.l.Debugf("channel read %#v", string(b))

	return b, nil
}

// ReadUntilFuzzy reads until a fuzzy match of the input is found.
func (c *Channel) ReadUntilFuzzy(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}

	var rb []byte

	for {
		nb, err := c.Read()
		if err != nil {
			return nil, err
		}

		if nb == nil {
			time.Sleep(c.ReadDelay)

			continue
		}

		rb = append(rb, nb...)

		if util.BytesRoughlyContains(
			b,
			processReadBuf(rb, getProcessReadBufSearchDepth(c.PromptSearchDepth, len(b))),
		) {
			return rb, nil
		}
	}
}

// ReadUntilExplicit reads bytes out of the channel Q object until the bytes b are seen in the
// output. Once the bytes are seen all read bytes are returned.
func (c *Channel) ReadUntilExplicit(b []byte) ([]byte, error) {
	var rb []byte

	for {
		nb, err := c.Read()
		if err != nil {
			return nil, err
		}

		if nb == nil {
			time.Sleep(c.ReadDelay)

			continue
		}

		rb = append(rb, nb...)

		if bytes.Contains(
			processReadBuf(rb, getProcessReadBufSearchDepth(c.PromptSearchDepth, len(b))),
			b,
		) {
			return rb, nil
		}
	}
}

// ReadUntilPrompt reads bytes out of the channel Q object until the channel PromptPattern regex
// pattern is seen in the output. Once that pattern is seen, all read bytes are returned.
func (c *Channel) ReadUntilPrompt(ctx context.Context) ([]byte, error) {
	var rb []byte

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		nb, err := c.Read()
		if err != nil {
			return nil, err
		}

		if nb == nil {
			time.Sleep(c.ReadDelay)

			continue
		}

		rb = append(rb, nb...)

		if c.PromptPattern.Match(processReadBuf(rb, c.PromptSearchDepth)) {
			return rb, nil
		}
	}
}

// ReadUntilAnyPrompt reads bytes out of the channel Q object until any of the prompts in the
// "prompts" argument are seen in the output. Once any pattern is seen, all read bytes are returned.
func (c *Channel) ReadUntilAnyPrompt(
	ctx context.Context,
	prompts []*regexp.Regexp,
) ([]byte, error) {
	var rb []byte

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		nb, err := c.Read()
		if err != nil {
			return nil, err
		}

		if nb == nil {
			time.Sleep(c.ReadDelay)

			continue
		}

		rb = append(rb, nb...)

		prb := processReadBuf(rb, c.PromptSearchDepth)

		for _, p := range prompts {
			if p.Match(prb) {
				return rb, nil
			}
		}
	}
}

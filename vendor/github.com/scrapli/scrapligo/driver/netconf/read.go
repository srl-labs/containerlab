package netconf

import (
	"bytes"
	"strconv"
	"time"
)

const (
	idOrSubMatchLen = 2
	endRPCSplitLen  = 2
)

func getID(match [][]byte) int {
	if len(match) != idOrSubMatchLen {
		return 0
	}

	id, _ := strconv.Atoi(string(match[1]))

	return id
}

func (d *Driver) read() {
	var b []byte

	patterns := getNetconfPatterns()

	for {
		select {
		case <-d.done:
			return
		default:
		}

		rb, err := d.Channel.Read()
		if err != nil {
			d.errs <- err
		}

		b = append(b, rb...)

		if d.Channel.PromptPattern.Match(b) { //nolint: nestif
			if bytes.Contains(b, []byte("</rpc>")) {
				// we read past the input, yay this is good, but we don't care that much, we just
				// need to reset the buffer... *but* because there is a small read delay in channel
				// we can sometimes already have read past the prompt/end of the original rpc. This
				// isn't an issue in "normal" SSH operations where we don't send return until we
				// read the input off the session, but obviously can break things here, so we'll
				// use regex to split on the delim and then get only the bits after the delim and
				// update b to be just that part.
				var ss []string

				switch d.SelectedVersion {
				case V1Dot0:
					ss = patterns.v1Dot0Delim.Split(string(b), endRPCSplitLen)
				case V1Dot1:
					ss = patterns.v1Dot1Delim.Split(string(b), endRPCSplitLen)
				}

				b = []byte(ss[1])
			} else if d.Channel.PromptPattern.Match(b) {
				var messageID int

				var subID int

				messageID = getID(patterns.messageID.FindSubmatch(b))

				if bytes.Contains(b, []byte("</subscription-id>")) {
					subID = getID(patterns.subscriptionID.FindSubmatch(b))
				}

				if messageID != 0 {
					d.Logger.Debugf(
						"Received message response for message ID '%d', storing", messageID,
					)

					d.storeMessage(messageID, b)
				}

				if subID != 0 {
					d.Logger.Debugf(
						"Received message response for subscription ID '%d', storing", subID,
					)

					d.storeSubscriptionMessage(subID, b)
				}

				b = nil
			}
		}

		time.Sleep(d.Channel.ReadDelay)
	}
}

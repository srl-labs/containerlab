package channel

// Write writes the given bytes b to the channel.
func (c *Channel) Write(b []byte, r bool) error {
	lm := string(b)
	if r {
		lm = redacted
	}

	c.l.Debugf("channel write %#v", lm)

	return c.t.Write(b)
}

// WriteReturn writes the channel ReturnChar to the channel.
func (c *Channel) WriteReturn() error {
	return c.Write(c.ReturnChar, false)
}

// WriteAndReturn writes the given bytes b and then sends the channel ReturnChar.
func (c *Channel) WriteAndReturn(b []byte, r bool) error {
	err := c.Write(b, r)
	if err != nil {
		return err
	}

	return c.WriteReturn()
}

package generic

// GetPrompt returns a string containing the current "prompt" of the connected ssh/telnet server.
func (d *Driver) GetPrompt() (string, error) {
	b, err := d.Channel.GetPrompt()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

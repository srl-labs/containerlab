package ciscoiosxr

// Cleanup is the platform implementation of Cleanup.
func (p *Platform) Cleanup() error {
	p.inProgress = false

	return nil
}

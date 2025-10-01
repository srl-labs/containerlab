package aristaeos

// Cleanup is the platform implementation of Cleanup.
func (p *Platform) Cleanup() error {
	var err error

	if p.candidateS != "" {
		err = p.DeRegisterConfigSession(p.candidateS)
	}

	return err
}

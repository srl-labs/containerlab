package ciscoiosxe

func (p *Platform) cleanConfigPayload(config string) string {
	idxs := p.patterns.outputHeader.FindStringIndex(config)

	if len(idxs) == 2 { //nolint:gomnd
		return config[idxs[0]:]
	}

	// we didn't find a header and/or the config is bad, but we'll let the device tell us the latter
	return config
}

// NormalizeConfig "normalizes" the configuration provided -- generally this means it replaces any
// kind of "header" pattern(s), but may vary from platform to platform.
func (p *Platform) NormalizeConfig(config string) string {
	return p.cleanConfigPayload(config)
}

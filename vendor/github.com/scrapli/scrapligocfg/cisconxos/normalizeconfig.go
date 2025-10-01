package cisconxos

import "strings"

// NormalizeConfig "normalizes" the configuration provided -- generally this means it replaces any
// kind of "header" pattern(s), but may vary from platform to platform.
func (p *Platform) NormalizeConfig(config string) string {
	config = p.patterns.outputHeader.ReplaceAllString(config, "")
	config = p.patterns.checkpointLine.ReplaceAllString(config, "")
	config = strings.Replace(config, "\n\n", "\n", -1)

	return config
}

package sros

const banner = `................................................................\n:                  Welcome to Nokia SR-OS!                     :\n:                                                              :\n:                                                              :\n: YANG:          https://yang.labctl.net/                      :\n: Community:     https://containerlab.dev/community/           :\n: Discord:       https://containerlab.dev/discord/             :\n................................................................\n`

// banner returns a banner string with a docs version filled in based on the version information queried from the node.
func (n *sros) banner() (string, error) {
	// if minor is a single digit value, we need to add extra space to patch version
	// to have banner table aligned nicely

	return banner, nil
}

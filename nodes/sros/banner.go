package sros

import (
	"fmt"
)

const banner = `................................................................\n:                  Welcome to Nokia SR-OS!                     :\n:                                                              :\n:                                                              :\n:                                                              :\n: Discord:     https://go.srlinux.dev/discord                  :\n: Contact:     https://go.srlinux.dev/contact-sales            :\n................................................................\n`

// banner returns a banner string with a docs version filled in based on the version information queried from the node.
func (n *sros) banner() (string, error) {
	// if minor is a single digit value, we need to add extra space to patch version
	// to have banner table aligned nicely

	b := fmt.Sprintf(banner)
	return b, nil
}

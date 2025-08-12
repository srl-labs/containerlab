package srl

import (
	"fmt"
)

const banner = `................................................................
:                  Welcome to Nokia SR Linux!                  :
:              Open Network OS for the NetOps era.             :
:                                                              :
:    This is a freely distributed official container image.    :
:                      Use it - Share it                       :
:                                                              :
: Get started: https://learn.srlinux.dev                       :
: Container:   https://go.srlinux.dev/container-image          :
: Docs:        https://doc.srlinux.dev/%s-%-2s                   :
: Rel. notes:  https://doc.srlinux.dev/rn%s-%s-%s               :
: YANG:        https://yang.srlinux.dev/v%s.%s.%s               :
: Discord:     https://go.srlinux.dev/discord                  :
: Contact:     https://go.srlinux.dev/contact-sales            :
................................................................
`

// banner returns a banner string with a docs version filled in based on the version information queried from the node.
func (n *srl) banner() (string, error) { //nolint: unparam
	// if minor is a single digit value, we need to add extra space to patch version
	// to have banner table aligned nicely
	if len(n.swVersion.Minor) == 1 {
		n.swVersion.Patch += " "
	}

	b := fmt.Sprintf(banner,
		n.swVersion.Major, n.swVersion.Minor,
		n.swVersion.Major, n.swVersion.Minor, n.swVersion.Patch,
		n.swVersion.Major, n.swVersion.Minor, n.swVersion.Patch)

	return b, nil
}

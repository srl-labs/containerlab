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
: Rel. notes:  https://doc.srlinux.dev/rnn                     :
: YANG:        https://yang.srlinux.dev/%-23s:
: Discord:     https://go.srlinux.dev/discord                  :
: Contact:     https://go.srlinux.dev/contact-sales            :
................................................................
`

// banner returns a banner string with a docs version filled in based on the version information
// queried from the node.
func (n *srl) banner() (string, error) { //nolint: unparam
	// The YANG line is rendered with a fixed-width field so the table stays
	// aligned regardless of how many digits the version components have
	// (e.g. x.y, xx.y or xx.yy).
	version := fmt.Sprintf("%s.%s.%s", n.swVersion.Major, n.swVersion.Minor, n.swVersion.Patch)

	b := fmt.Sprintf(banner,
		n.swVersion.Major, n.swVersion.Minor, // Docs
		version) // YANG

	return b, nil
}

package types

import (
	"fmt"
	"strings"
)

// LinkBrief is the representation of any supported link in a brief format as defined in the topology file.
type LinkBrief struct {
	Endpoints        []string
	LinkCommonParams `yaml:",inline"`
}

// Resolve resolves the brief link into a concrete RawLink implementation.
// LinkBrief is only used to have a short version of a link definition in the topology file,
// with Resolve we convert it into one of the supported link types.
func (l *LinkBrief) Resolve() (RawLink, error) {
	// check two endpoints defined
	if len(l.Endpoints) != 2 {
		return nil, fmt.Errorf("endpoint definition should consist of exactly 2 entries. %d provided", len(l.Endpoints))
	}
	for x, v := range l.Endpoints {
		parts := strings.SplitN(v, ":", 2)
		node := parts[0]

		lt, err := parseLinkType(node)
		if err != nil {
			continue
		}

		switch lt {
		case LinkTypeMacVLan:
			return macVlanLinkFromBrief(l, x)
		case LinkTypeMgmtNet:
			return mgmtNetLinkFromBrief(l, x)
		case LinkTypeHost:
			return hostLinkFromBrief(l, x)
		}
	}

	return vEthFromLinkConfig(l)
}

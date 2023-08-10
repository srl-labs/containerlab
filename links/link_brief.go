package links

import (
	"fmt"
	"strings"
)

// LinkBriefRaw is the representation of any supported link in a brief format as defined in the topology file.
type LinkBriefRaw struct {
	Endpoints        []string `yaml:"endpoints"`
	LinkCommonParams `yaml:",inline,omitempty"`
}

// ToRawLink resolves the brief link into a concrete RawLink implementation.
// LinkBrief is only used to have a short version of a link definition in the topology file,
// with ToRawLink we convert it into one of the supported link types.
func (l *LinkBriefRaw) ToTypeSpecificRawLink() (RawLink, error) {
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

func (l *LinkBriefRaw) GetType() LinkType {
	return LinkTypeBrief
}

func (l *LinkBriefRaw) Resolve(params *ResolveParams) (Link, error) {
	return nil, fmt.Errorf("resolve unimplemented on LinkBriefRaw. Use <LinkBriefRaw>.ToTypeSpecificRawLink() and call resolve on the result")
}

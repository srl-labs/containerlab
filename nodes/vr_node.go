package nodes

import (
	"fmt"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/links"
)

const FirstEthDataInterfaceIndex = 1

var VMInterfaceRegexp = regexp.MustCompile(`eth[1-9][0-9]*$`) // skipcq: GO-C4007

type VRNode struct {
	DefaultNode
	OverwriteVRNode            VRNodeOverwrites
	InterfaceRegexp            *regexp.Regexp
	InterfaceOffset            int
	InterfaceHelp              string
	FirstEthDataInterfaceIndex int
}

type VRNodeOverwrites interface {
	CalculateInterfaceIndex(ifName string) (int, error)
	GetMappedInterfaceName(ifName string) (string, error)
}

func NewVRNode(n VRNodeOverwrites) *VRNode {
	vrn := &VRNode{
		OverwriteVRNode: n,
	}

	vrn.DefaultNode = *NewDefaultNode(n.(NodeOverwrites))
	vrn.InterfaceOffset = 0
	vrn.FirstEthDataInterfaceIndex = 1

	return vrn
}

// CalculateInterfaceIndex parses the supplied interface name with the InterfaceRegexp.
// Using the the port offset, it calculates the ethX interface index based on the InterfaceOffset and the first data interface index.
func (vr *VRNode) CalculateInterfaceIndex(ifName string) (int, error) {
	matches := vr.InterfaceRegexp.FindStringSubmatch(ifName)
	if len(matches) == 0 {
		return 0, fmt.Errorf("%q does not match interface alias regexp %q, no match", ifName, vr.InterfaceRegexp)
	}

	captureGroups := make(map[string]string)
	for i, name := range vr.InterfaceRegexp.SubexpNames() {
		if i != 0 && name != "" {
			captureGroups[name] = matches[i]
		}
	}

	if parsedIndex, found := captureGroups["port"]; found {
		parsedIndexInt, err := strconv.Atoi(parsedIndex)
		if err != nil {
			return 0, fmt.Errorf("%q parsed index %q could not be cast to an integer", ifName, parsedIndex)
		}
		calculatedIndex := parsedIndexInt - vr.InterfaceOffset + vr.FirstEthDataInterfaceIndex
		return calculatedIndex, nil
	} else {
		return 0, fmt.Errorf("%q does not have extracted interface index with regexp %q, 'port' capture group missing?", ifName, vr.InterfaceRegexp)
	}
}

// GetMappedInterfaceName returns with the mapped ethX name if the interface alias regexp is defined.
// If the interface name does not match the regexp, try to match it with the generic VM interface regexp. If both fail, return with an error.
func (vr *VRNode) GetMappedInterfaceName(ifName string) (string, error) {
	ifIndex, err := vr.OverwriteVRNode.CalculateInterfaceIndex(ifName)
	if err != nil {
		return "", err
	}
	if !(ifIndex >= 1) {
		return "", fmt.Errorf("extracted interface index for %q is out of bounds: %d ! >= 1", ifName, ifIndex)
	}
	mappedIfName := fmt.Sprintf("eth%d", ifIndex)

	return mappedIfName, nil
}

// AddEndpoint's vrnetlab version maps the endpoint name to an ethX-based name before adding it to the node endpoints. Returns an error if the mapping goes wrong.
func (vr *VRNode) AddEndpoint(e links.Endpoint) error {
	endpointName := e.GetIfaceName()
	if vr.InterfaceRegexp != nil && !(VMInterfaceRegexp.MatchString(endpointName)) {
		mappedName, err := vr.OverwriteVRNode.GetMappedInterfaceName(endpointName)
		if err != nil {
			return fmt.Errorf("%q interface name %q could not be mapped to an ethX-based interface name: %w", vr.Cfg.ShortName, e.GetIfaceName(), err)
		}
		log.Debugf("Interface Mapping: Mapping interface %q (ifAlias) to %q (ifName)", endpointName, mappedName)
		e.SetIfaceName(mappedName)
		e.SetIfaceAlias(endpointName)
	}
	vr.Endpoints = append(vr.Endpoints, e)

	return nil
}

// CheckInterfaceName checks interface names for generic VM-based nodes.
// Displays InterfaceHelp if the check fails for interface naming hints.
func (vr *VRNode) CheckInterfaceName() error {
	var seenEps []links.Endpoint

	for _, ep := range vr.Endpoints {
		ifName := ep.GetIfaceName()
		for _, seenEp := range seenEps {
			if ifName == seenEp.GetIfaceName() {
				return fmt.Errorf("%q (%q) and %q (%q) have overlapping interface names", ifName, ep.GetIfaceAlias(), seenEp.GetIfaceName(), seenEp.GetIfaceAlias())
			}
		}
		seenEps = append(seenEps, ep)
		if !VMInterfaceRegexp.MatchString(ifName) {
			return fmt.Errorf("%q interface name %q doesn't match the required interface patterns: %q", vr.Cfg.ShortName, ifName, vr.InterfaceHelp)
		}
	}

	return nil
}

package nodes

import (
	"fmt"
	"regexp"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/links"
)

var VMInterfaceRegexp = regexp.MustCompile(`eth[1-9][0-9]*$`) // skipcq: GO-C4007

type VRNode struct {
	DefaultNode
}

func NewVRNode(n NodeOverwrites) *VRNode {
	vr := &VRNode{}

	vr.DefaultNode = *NewDefaultNode(n)

	vr.InterfaceMappedPrefix = "eth"
	vr.InterfaceOffset = 0
	vr.FirstDataIfIndex = 1

	return vr
}

// AddEndpoint override version maps the endpoint name to an ethX-based name before adding it to the node endpoints. Returns an error if the mapping goes wrong.
func (vr *VRNode) AddEndpoint(e links.Endpoint) error {
	endpointName := e.GetIfaceName()
	// Slightly modified check: if it doesn't match the VMInterfaceRegexp, pass it to GetMappedInterfaceName. If it fails, then the interface name is wrong.
	if vr.InterfaceRegexp != nil && !(VMInterfaceRegexp.MatchString(endpointName)) {
		mappedName, err := vr.OverwriteNode.GetMappedInterfaceName(endpointName)
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
// Displays InterfaceHelp if the check fails for the expected VM interface regexp.
func (vr *VRNode) CheckInterfaceName() error {
	err := vr.CheckInterfaceOverlap()
	if err != nil {
		return err
	}

	for _, ep := range vr.Endpoints {
		ifName := ep.GetIfaceName()
		if !VMInterfaceRegexp.MatchString(ifName) {
			return fmt.Errorf("%q interface name %q does not match the required interface patterns: %q", vr.Cfg.ShortName, ifName, vr.InterfaceHelp)
		}
	}

	return nil
}

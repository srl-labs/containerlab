package links

import (
	"fmt"
	"net"
	"net/netip"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// EndpointRaw is the raw (string) representation of an endpoint as defined in the topology file
// for a given link definition.
type EndpointRaw struct {
	Node  string         `yaml:"node"`
	Iface string         `yaml:"interface"`
	MAC   string         `yaml:"mac,omitempty"`
	IPv4  string         `yaml:"ipv4,omitempty"`
	IPv6  string         `yaml:"ipv6,omitempty"`
	Vars  map[string]any `yaml:"vars,omitempty"`
}

// NewEndpointRaw creates a new EndpointRaw struct.
func NewEndpointRaw(node, nodeIf, mac string) *EndpointRaw {
	return &EndpointRaw{
		Node:  node,
		Iface: nodeIf,
		MAC:   mac,
	}
}

// Resolve resolves the EndpointRaw into an Endpoint interface that is implemented
// by a concrete endpoint struct such as EndpointBridge, EndpointHost, EndpointVeth.
// The type of an endpoint is determined by the node it belongs to.
// Resolving a raw endpoint adds an associated Link and Node to the endpoint.
// It also adds the endpoint to the node.
func (er *EndpointRaw) Resolve(params *ResolveParams, l Link) (Endpoint, error) {
	// check if the referenced node does exist
	node, exists := params.Nodes[er.Node]
	if !exists {
		return nil, fmt.Errorf("unable to find node %s", er.Node)
	}

	genericEndpoint := NewEndpointGeneric(node, er.Iface, l)

	if er.IPv4 != "" {
		p, err := netip.ParsePrefix(er.IPv4)
		if err != nil || !p.Addr().Is4() {
			return nil, fmt.Errorf(
				"invalid ipv4 address %q for %s:%s",
				er.IPv4,
				er.Node,
				er.Iface,
			)
		}
		genericEndpoint.IPv4 = p
	}

	if er.IPv6 != "" {
		p, err := netip.ParsePrefix(er.IPv6)
		if err != nil || !p.Addr().Is6() {
			return nil, fmt.Errorf(
				"invalid ipv6 address %q for %s:%s",
				er.IPv6,
				er.Node,
				er.Iface,
			)
		}
		genericEndpoint.IPv6 = p
	}

	if er.Vars != nil {
		// Normalize vars to ensure JSON serialization compatibility
		// (converts map[interface{}]interface{} to map[string]any)
		normalizedVars := make(map[string]any)
		for k, v := range er.Vars {
			normalizedVars[k] = clabutils.NormalizeMapForJSON(v)
		}
		genericEndpoint.Vars = normalizedVars
	}

	var err error
	if er.MAC == "" {
		// if mac is not present generate one
		genericEndpoint.MAC, err = clabutils.GenMac(clabconstants.ClabOUI)
		if err != nil {
			return nil, err
		}
	} else {
		// if MAC is present, set it
		m, err := net.ParseMAC(er.MAC)
		if err != nil {
			return nil, err
		}
		genericEndpoint.MAC = m
	}

	var e Endpoint

	switch node.GetLinkEndpointType() {
	case LinkEndpointTypeBridge, LinkEndpointTypeBridgeNS:
		e = NewEndpointBridge(genericEndpoint, false)

	case LinkEndpointTypeHost:
		e = NewEndpointHost(genericEndpoint)

	case LinkEndpointTypeVeth:
		e = NewEndpointVeth(genericEndpoint)
	}
	if l.GetType() == LinkTypeDummy {
		e = NewEndpointDummy(genericEndpoint)
	}
	// also add the endpoint to the node
	err = node.AddEndpoint(e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

package links

import (
	"fmt"
	"net"

	"github.com/srl-labs/containerlab/utils"
)

// EndpointRaw is the raw (string) representation of an endpoint as defined in the topology file
// for a given link definition.
type EndpointRaw struct {
	Node      string    `yaml:"node"`
	Iface     string    `yaml:"interface"`
	MAC       string    `yaml:"mac,omitempty"`
	OperState OperState `yaml:"oper-state"`
}

// NewEndpointRaw creates a new EndpointRaw struct.
func NewEndpointRaw(node, nodeIf, mac string) *EndpointRaw {
	return &EndpointRaw{
		Node:      node,
		Iface:     nodeIf,
		MAC:       mac,
		OperState: up,
	}
}

// UnmarshalYAML for EndpointRaw
func (s *EndpointRaw) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First, unmarshal the entire object into the struct
	type Alias EndpointRaw // Create an alias to avoid infinite recursion
	aux := (*Alias)(s)

	// Unmarshal the YAML into the struct
	if err := unmarshal(&aux); err != nil {
		return err
	}

	// Set the default value for OperState if it's empty
	if s.OperState == "" {
		s.OperState = up
	}

	return nil
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

	genericEndpoint := NewEndpointGeneric(node, er.Iface, er.OperState, l)

	var err error
	if er.MAC == "" {
		// if mac is not present generate one
		genericEndpoint.MAC, err = utils.GenMac(ClabOUI)
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
	case LinkEndpointTypeBridge:
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

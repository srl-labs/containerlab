package links

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// EndpointRaw is the raw (string) representation of an endpoint as defined in the topology file
// for a given link definition.
type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	MAC   string `yaml:"mac,omitempty"`
}

// NewEndpointRaw creates a new EndpointRaw struct.
func NewEndpointRaw(node, nodeIf, Mac string) *EndpointRaw {
	return &EndpointRaw{
		Node:  node,
		Iface: nodeIf,
		MAC:   Mac,
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

	genericEndpoint := &EndpointGeneric{
		Node:      node,
		IfaceName: er.Iface,
		Link:      l,
	}

	// if MAC is present, set it
	if er.MAC != "" {
		m, err := net.ParseMAC(er.MAC)
		if err != nil {
			return nil, err
		}
		genericEndpoint.MAC = m
	}

	var e Endpoint

	switch node.GetLinkEndpointType() {
	case LinkEndpointTypeBridge:
		e = &EndpointBridge{
			EndpointGeneric: *genericEndpoint,
			masterInterface: node.GetShortName(),
		}
	case LinkEndpointTypeHost:
		e = &EndpointHost{
			EndpointGeneric: *genericEndpoint,
		}
	case LinkEndpointTypeVeth:
		e = &EndpointVeth{
			EndpointGeneric: *genericEndpoint,
		}
	}

	// also add the endpoint to the node
	err := node.AddEndpoint(e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

// EndpointGeneric is the generic endpoint struct that is used by all endpoint types.
type EndpointGeneric struct {
	Node      Node
	IfaceName string
	// Link is the link this endpoint belongs to.
	Link     Link
	MAC      net.HardwareAddr
	randName string
	state    EndpointDeployState
}

func (e *EndpointGeneric) GetRandIfaceName() string {
	// generate random interface name on the fly if not already generated
	if e.randName == "" {
		e.randName = genRandomIfName()
	}
	return e.randName
}

func (e *EndpointGeneric) GetIfaceName() string {
	return e.IfaceName
}

func (e *EndpointGeneric) GetState() EndpointDeployState {
	return e.state
}

func (e *EndpointGeneric) GetMac() net.HardwareAddr {
	return e.MAC
}

func (e *EndpointGeneric) GetLink() Link {
	return e.Link
}

func (e *EndpointGeneric) GetNode() Node {
	return e.Node
}

func (e *EndpointGeneric) IsSameNodeInterface(ept Endpoint) bool {
	return e.Node == ept.GetNode() && e.IfaceName == ept.GetIfaceName()
}

func (e *EndpointGeneric) Deploy(ctx context.Context) error {
	e.state = EndpointDeployStateReady
	return e.Link.Deploy(ctx)
}

func (e *EndpointGeneric) String() string {
	return fmt.Sprintf("%s:%s", e.Node.GetShortName(), e.IfaceName)
}

type EndpointDeployState uint8

const (
	EndpointDeployStateNotReady = iota
	EndpointDeployStateReady
	EndpointDeployStateDeployed
)

// Endpoint is the interface that all endpoint types implement.
// Endpoints like bridge, host, veth and macvlan are the types implementing this interface.
type Endpoint interface {
	GetNode() Node
	GetIfaceName() string
	GetRandIfaceName() string
	GetMac() net.HardwareAddr
	Deploy(ctx context.Context) error
	String() string
	// GetLink retrieve the link that the endpoint is assiged to
	GetLink() Link
	// Verify is used to verify the endpoint with all its
	// dependencies. The Endpt slice contains all the Endpoints
	// of the topology
	Verify([]Endpoint) error
	// IsSameNodeInterface is the equal check for two endpoints that
	// does take the node and the Interfacename into account
	IsSameNodeInterface(ept Endpoint) bool
	GetState() EndpointDeployState
}

type EndpointBridge struct {
	EndpointGeneric
	masterInterface string
}

func (e *EndpointBridge) Verify(_ []Endpoint) error {
	errs := []error{}
	err := CheckPerNodeInterfaceUniqueness(e)
	if err != nil {
		errs = append(errs, err)
	}
	err = CheckBridgeExists(e.GetNode(), e.masterInterface)
	if err != nil {
		errs = append(errs, err)
	}
	err = CheckEndpointDoesNotExistYet(e)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

type EndpointHost struct {
	EndpointGeneric
}

func (e *EndpointHost) Verify(_ []Endpoint) error {
	errs := []error{}
	err := CheckPerNodeInterfaceUniqueness(e)
	if err != nil {
		errs = append(errs, err)
	}
	err = CheckEndpointDoesNotExistYet(e)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

type EndptMacVlan struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndptMacVlan) Verify(_ []Endpoint) error {
	return CheckEndptExists(e)
}

type EndpointVeth struct {
	EndpointGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndpointVeth) Verify(_ []Endpoint) error {
	return CheckPerNodeInterfaceUniqueness(e)
}

// CheckPerNodeInterfaceUniqueness takes a specific Endpt and a slice of Endpts as input and verifies, that for the node referenced in the given Endpt,
func CheckPerNodeInterfaceUniqueness(e Endpoint) error {
	for _, ept := range e.GetNode().GetEndpoints() {
		if e == ept {
			// epts contains all endpoints, hence also the
			// one we're checking here. So if ept is pointer equal to e,
			// we continue with next ept
			continue
		}
		// check if the two Endpts are equal
		if e.IsSameNodeInterface(ept) {
			return fmt.Errorf("duplicate endpoint %s", e.String())
		}
	}
	return nil
}

// CheckEndptExists is the low level function to check that a certain
// interface exists in the network namespace of the given node
func CheckEndptExists(e Endpoint) error {
	err := CheckEndpointDoesNotExistYet(e)
	if err == nil {
		return fmt.Errorf("interface %q does not exist", e.String())
	}
	return nil
}

// CheckBridgeExists verifies that the given bridge is present in the
// netnwork namespace referenced via the provided nspath handle
func CheckBridgeExists(n Node, brName string) error {
	return n.ExecFunction(func(_ ns.NetNS) error {
		br, err := netlink.LinkByName(brName)
		_, notfound := err.(netlink.LinkNotFoundError)
		switch {
		case notfound:
			return fmt.Errorf("bridge %q referenced in topology but does not exist", brName)
		case err != nil:
			return err
		case br.Type() != "bridge":
			return fmt.Errorf("interface %s found. expected type \"bridge\", actual is %q", brName, br.Type())
		}
		return nil
	})
}

// CheckEndpointDoesNotExistYet verifies that the interface referenced in the
// provided endpoint does not yet exist in the referenced node.
func CheckEndpointDoesNotExistYet(e Endpoint) error {
	return e.GetNode().ExecFunction(func(_ ns.NetNS) error {
		// we expect a netlink.LinkNotFoundError when querying for
		// the interface with the given endpoints name
		_, err := netlink.LinkByName(e.GetIfaceName())
		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}

		return fmt.Errorf("interface %s is defined via topology but does already exist", e.String())
	})
}

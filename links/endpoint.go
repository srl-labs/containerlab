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
	Mac   string `yaml:"mac,omitempty"`
}

// NewEndpointRaw creates a new EndpointRaw struct.
func NewEndpointRaw(node, nodeIf, Mac string) *EndpointRaw {
	return &EndpointRaw{
		Node:  node,
		Iface: nodeIf,
		Mac:   Mac,
	}
}

func (e *EndpointRaw) Resolve(params *ResolveParams, l Link) (Endpoint, error) {
	// check if the referenced node does exist
	node, exists := params.Nodes[e.Node]
	if !exists {
		return nil, fmt.Errorf("unable to find node %s", e.Node)
	}

	// create the result struct
	genericEndpt := &EndptGeneric{
		Node:  node,
		Iface: e.Iface,
		Link:  l,
	}

	// if MAC is present, set it
	if e.Mac != "" {
		m, err := net.ParseMAC(e.Mac)
		if err != nil {
			return nil, err
		}
		genericEndpt.Mac = m
	}

	var finalEndpt Endpoint

	switch node.GetLinkEndpointType() {
	case LinkEndpointTypeBridge:
		finalEndpt = &EndptBridge{
			EndptGeneric:    *genericEndpt,
			masterInterface: node.GetShortName(),
		}
	case LinkEndpointTypeHost:
		finalEndpt = &EndptHost{
			EndptGeneric: *genericEndpt,
		}
	case LinkEndpointTypeRegular:
		finalEndpt = &EndptVeth{
			EndptGeneric: *genericEndpt,
		}
	}

	// also add the endpoint to the node
	err := node.AddEndpoint(finalEndpt)
	if err != nil {
		return nil, err
	}

	return finalEndpt, nil
}

type EndptGeneric struct {
	Node     LinkNode
	Iface    string
	Link     Link
	Mac      net.HardwareAddr
	randName string
	state    EndptDeployState
}

func (e *EndptGeneric) GetRandIfaceName() string {
	// generate random interface name on the fly if not already generated
	if e.randName == "" {
		e.randName = genRandomIfName()
	}
	return e.randName
}

func (e *EndptGeneric) GetIfaceName() string {
	return e.Iface
}

func (e *EndptGeneric) GetState() EndptDeployState {
	return e.state
}

func (e *EndptGeneric) GetMac() net.HardwareAddr {
	return e.Mac
}

func (e *EndptGeneric) GetLink() Link {
	return e.Link
}

func (e *EndptGeneric) GetNode() LinkNode {
	return e.Node
}

func (e *EndptGeneric) IsSameNodeInterface(ept Endpoint) bool {
	return e.Node == ept.GetNode() && e.Iface == ept.GetIfaceName()
}

func (e *EndptGeneric) Deploy(ctx context.Context) error {
	e.state = EndptDeployStateReady
	return e.Link.Deploy(ctx)
}

func (e *EndptGeneric) String() string {
	return fmt.Sprintf("%s:%s", e.Node.GetShortName(), e.Iface)
}

type EndptDeployState int8

const (
	EndptDeployStateNotReady = iota
	EndptDeployStateReady
	EndptDeployStateDeployed
)

// Endpoint is the interface that all endpoint types implement.
// Endpoints like bridge, host, veth and macvlan are the types implementing this interface.
type Endpoint interface {
	GetNode() LinkNode
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
	GetState() EndptDeployState
}

type EndptBridge struct {
	EndptGeneric
	masterInterface string
}

func (e *EndptBridge) Verify(_ []Endpoint) error {
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

type EndptHost struct {
	EndptGeneric
}

func (e *EndptHost) Verify(_ []Endpoint) error {
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
	EndptGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndptMacVlan) Verify(_ []Endpoint) error {
	return CheckEndptExists(e)
}

type EndptVeth struct {
	EndptGeneric
}

// Verify verifies the veth based deployment pre-conditions
func (e *EndptVeth) Verify(_ []Endpoint) error {
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
func CheckBridgeExists(n LinkNode, brName string) error {
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

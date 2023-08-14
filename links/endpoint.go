package links

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const (
	// containerlab's reserved OUI.
	ClabOUI = "aa:c1:ab"
)

// Endpoint is the interface that all endpoint types implement.
// Endpoints like bridge, host, veth and macvlan are the types implementing this interface.
type Endpoint interface {
	GetNode() Node
	GetIfaceName() string
	GetRandIfaceName() string
	GetMac() net.HardwareAddr
	String() string
	// GetLink retrieves the link that the endpoint is assigned to
	GetLink() Link
	// Verify verifies that the endpoint is valid and can be deployed
	Verify(*VerifyLinkParams) error
	// HasSameNodeAndInterface returns true if an endpoint that implements this interface
	// has the same node and interface name as the given endpoint.
	HasSameNodeAndInterface(ept Endpoint) bool
	Remove() error
}

// EndpointGeneric is the generic endpoint struct that is used by all endpoint types.
type EndpointGeneric struct {
	Node      Node
	IfaceName string
	// Link is the link this endpoint belongs to.
	Link     Link
	MAC      net.HardwareAddr
	randName string
}

func NewEndpointGeneric(node Node, iface string) *EndpointGeneric {
	return &EndpointGeneric{
		Node:      node,
		IfaceName: iface,
		// random name is generated for the endpoint to avoid name collisions
		// when it is first deployed in the root namespace
		randName: genRandomIfName(),
	}
}

func (e *EndpointGeneric) GetRandIfaceName() string {
	return e.randName
}

func (e *EndpointGeneric) GetIfaceName() string {
	return e.IfaceName
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

func (e *EndpointGeneric) Remove() error {
	return e.GetNode().ExecFunction(func(_ ns.NetNS) error {
		brSideEp, err := netlink.LinkByName(e.GetIfaceName())
		_, notfound := err.(netlink.LinkNotFoundError)

		switch {
		case notfound:
			// interface is not present, all good
			return nil
		case err != nil:
			return err
		}

		return netlink.LinkDel(brSideEp)
	})
}

// HasSameNodeAndInterface returns true if the given endpoint has the same node and interface name
// as the `ept` endpoint.
func (e *EndpointGeneric) HasSameNodeAndInterface(ept Endpoint) bool {
	return e.Node == ept.GetNode() && e.IfaceName == ept.GetIfaceName()
}

func (e *EndpointGeneric) String() string {
	return fmt.Sprintf("%s:%s", e.Node.GetShortName(), e.IfaceName)
}

// CheckEndpointUniqueness checks that the given endpoint appears only once for the node
// it is assigned to.
func CheckEndpointUniqueness(e Endpoint) error {
	for _, ept := range e.GetNode().GetEndpoints() {
		if e == ept {
			// since node contains all endpoints including the one we are checking
			// we skip it
			continue
		}
		// if `e` has the same node and interface name as `ept` then we have a duplicate
		if e.HasSameNodeAndInterface(ept) {
			return fmt.Errorf("duplicate endpoint %s", e)
		}
	}

	return nil
}

// CheckEndpointExists checks that a certain
// interface exists in the network namespace of the given node.
func CheckEndpointExists(e Endpoint) error {
	err := CheckEndpointDoesNotExistYet(e)
	if err == nil {
		return fmt.Errorf("interface %q does not exist", e.String())
	}
	return nil
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

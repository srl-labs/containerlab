package links

import (
	"context"
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
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
	GetIfaceAlias() string
	GetIfaceDisplayName() string
	GetRandIfaceName() string
	GetMac() net.HardwareAddr
	String() string
	// GetLink retrieves the link that the endpoint is assigned to
	GetLink() Link
	// Verify verifies that the endpoint is valid and can be deployed
	Verify(context.Context, *VerifyLinkParams) error
	// HasSameNodeAndInterface returns true if an endpoint that implements this interface
	// has the same node and interface name as the given endpoint.
	HasSameNodeAndInterface(ept Endpoint) bool
	Remove(context.Context) error
	// Deploy deploys the endpoint by calling the Deploy method of the link it is assigned to
	// and passing the endpoint as an argument so that the link that consists of A and B endpoints
	// can deploy them independently.
	Deploy(context.Context) error
	// IsNodeless returns true for the endpoints that has no explicit node defined in the topology.
	// E.g. host endpoints, mgmt bridge endpoints.
	// Because there is no node that would deploy this side of the link they should be deployed along
	// with the A side of the veth link.
	IsNodeless() bool
	// Setters for ifaceName and Alias
	SetIfaceName(string)
	SetIfaceAlias(string)
}

// EndpointGeneric is the generic endpoint struct that is used by all endpoint types.
type EndpointGeneric struct {
	Node       Node
	IfaceName  string
	IfaceAlias string
	// Link is the link this endpoint belongs to.
	Link     Link
	MAC      net.HardwareAddr
	randName string
}

func NewEndpointGeneric(node Node, iface string, link Link) *EndpointGeneric {
	return &EndpointGeneric{
		Node:       node,
		IfaceName:  iface,
		IfaceAlias: "",
		// random name is generated for the endpoint to avoid name collisions
		// when it is first deployed in the root namespace
		randName: genRandomIfName(),
		Link:     link,
	}
}

func (e *EndpointGeneric) GetRandIfaceName() string {
	return e.randName
}

func (e *EndpointGeneric) GetIfaceName() string {
	return e.IfaceName
}

func (e *EndpointGeneric) GetIfaceAlias() string {
	return e.IfaceAlias
}

func (e *EndpointGeneric) GetIfaceDisplayName() string {
	if e.IfaceAlias != "" {
		return e.IfaceAlias
	}
	return e.IfaceName
}

func (e *EndpointGeneric) SetIfaceName(ifaceName string) {
	e.IfaceName = ifaceName
}

func (e *EndpointGeneric) SetIfaceAlias(ifaceAlias string) {
	e.IfaceAlias = ifaceAlias
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

func (e *EndpointGeneric) Remove(ctx context.Context) error {
	return e.GetNode().ExecFunction(ctx, func(n ns.NetNS) error {
		brSideEp, err := netlink.LinkByName(e.GetIfaceName())
		_, notfound := err.(netlink.LinkNotFoundError)

		switch {
		case notfound:
			// interface is not present, all good
			return nil
		case err != nil:
			return err
		}
		log.Debugf("Removing interface %q from namespace %q", e.GetIfaceName(), e.GetNode().GetShortName())
		return netlink.LinkDel(brSideEp)
	})
}

// HasSameNodeAndInterface returns true if the given endpoint has the same node and interface name
// as the `ept` endpoint.
func (e *EndpointGeneric) HasSameNodeAndInterface(ept Endpoint) bool {
	return e.Node == ept.GetNode() && e.IfaceName == ept.GetIfaceName()
}

func (e *EndpointGeneric) String() string {
	ifDisplayName := e.IfaceName
	if e.IfaceAlias != "" {
		ifDisplayName += fmt.Sprintf(" (%s)", e.IfaceAlias)
	}
	return fmt.Sprintf("%s:%s", e.Node.GetShortName(), ifDisplayName)
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
func CheckEndpointExists(ctx context.Context, e Endpoint) error {
	err := CheckEndpointDoesNotExistYet(ctx, e)
	if err == nil {
		return fmt.Errorf("interface %q does not exist", e.String())
	}
	return nil
}

// CheckEndpointDoesNotExistYet verifies that the interface referenced in the
// provided endpoint does not yet exist in the referenced node.
func CheckEndpointDoesNotExistYet(ctx context.Context, e Endpoint) error {
	return e.GetNode().ExecFunction(ctx, func(_ ns.NetNS) error {
		// we expect a netlink.LinkNotFoundError when querying for
		// the interface with the given endpoints name
		var err error
		_, err = netlink.LinkByName(e.GetIfaceName())

		if _, notfound := err.(netlink.LinkNotFoundError); notfound {
			return nil
		}

		return fmt.Errorf("interface %s is defined via topology but does already exist: %v", e.String(), err)
	})
}

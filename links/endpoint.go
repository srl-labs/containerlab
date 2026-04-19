package links

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

// Endpoint is the interface that all endpoint types implement.
// Endpoints like bridge, host, veth and macvlan are the types implementing this interface.
type Endpoint interface {
	GetNode() Node
	SetNode(Node)
	GetIfaceName() string
	GetIfaceAlias() string
	GetIfaceDisplayName() string
	GetRandIfaceName() string
	GetMac() net.HardwareAddr
	GetIPv4Addr() netip.Prefix
	GetIPv6Addr() netip.Prefix
	String() string
	// GetLink retrieves the link that the endpoint is assigned to
	GetLink() Link
	// Verify verifies that the endpoint is valid and can be deployed
	Verify(context.Context, *VerifyLinkParams) error
	// HasSameNodeAndInterface returns true if an endpoint that implements this interface
	// has the same node and interface name as the given endpoint.
	HasSameNodeAndInterface(ept Endpoint) bool
	Remove(context.Context) error
	// IsNodeless returns true for the endpoints that has no explicit node defined in the topology.
	// E.g. host endpoints, mgmt bridge endpoints.
	// Because there is no node that would deploy this side of the link they should be deployed
	// along
	// with the A side of the veth link.
	IsNodeless() bool
	// Setters for ifaceName and Alias
	SetIfaceName(string)
	SetIfaceAlias(string)
	IsRuntimeDiscovered() bool
	MoveTo(context.Context, Node, bool) error
	// GetVars returns the endpoint-level vars.
	GetVars() map[string]any
}

// DeployableEndpoint is implemented by endpoint kinds that participate in initial lab deployment.
type DeployableEndpoint interface {
	Endpoint
	// Deploy deploys the endpoint by calling the Deploy method of the link it is assigned to
	// and passing the endpoint as an argument so that the link that consists of A and B endpoints
	// can deploy them independently.
	Deploy(context.Context) error
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
	IPv4     netip.Prefix
	IPv6     netip.Prefix
	Vars     map[string]any
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
		Vars:     make(map[string]any),
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

func (e *EndpointGeneric) GetIPv4Addr() netip.Prefix {
	return e.IPv4
}

func (e *EndpointGeneric) GetIPv6Addr() netip.Prefix {
	return e.IPv6
}

func (e *EndpointGeneric) GetVars() map[string]any {
	return e.Vars
}

func (e *EndpointGeneric) GetLink() Link {
	return e.Link
}

func (e *EndpointGeneric) GetNode() Node {
	return e.Node
}

func (e *EndpointGeneric) SetNode(node Node) {
	e.Node = node
}

func (*EndpointGeneric) IsRuntimeDiscovered() bool {
	return false
}

func (e *EndpointGeneric) Remove(ctx context.Context) error {
	return e.GetNode().ExecFunction(ctx, func(ns.NetNS) error {
		brSideEp, err := netlink.LinkByName(e.GetIfaceName())
		_, notfound := err.(netlink.LinkNotFoundError)

		switch {
		case notfound:
			// interface is not present, all good
			return nil
		case err != nil:
			return err
		}
		log.Debugf(
			"Removing interface %q from namespace %q",
			e.GetIfaceName(),
			e.GetNode().GetShortName(),
		)
		return netlink.LinkDel(brSideEp)
	})
}

func moveEndpoint(ctx context.Context, e Endpoint, dst Node, bringUp bool) error {
	src := e.GetNode()
	if src == nil {
		return fmt.Errorf("endpoint %q has no source node", e.GetIfaceName())
	}

	if dst == nil {
		return fmt.Errorf("endpoint %q has no destination node", e.GetIfaceName())
	}

	if src == dst {
		if !bringUp {
			return nil
		}

		return src.ExecFunction(ctx, func(_ ns.NetNS) error {
			link, err := netlink.LinkByName(e.GetIfaceName())
			if err != nil {
				return err
			}

			return netlink.LinkSetUp(link)
		})
	}

	srcOwner, ok := src.(EndpointOwner)
	if !ok {
		return fmt.Errorf("node %q does not support endpoint ownership moves", src.GetShortName())
	}

	dstOwner, ok := dst.(EndpointOwner)
	if !ok {
		return fmt.Errorf("node %q does not support endpoint ownership moves", dst.GetShortName())
	}

	srcOwnsEndpoint := false
	for _, owned := range src.GetEndpoints() {
		if owned == e {
			srcOwnsEndpoint = true
			break
		}
	}
	if !srcOwnsEndpoint {
		return fmt.Errorf("node %q does not own endpoint %q", src.GetShortName(), e.GetIfaceName())
	}

	for _, owned := range dst.GetEndpoints() {
		if owned == e {
			return fmt.Errorf("node %q already owns endpoint %q", dst.GetShortName(), e.GetIfaceName())
		}
		if owned.GetIfaceName() == e.GetIfaceName() {
			return fmt.Errorf(
				"node %q already tracks interface %q",
				dst.GetShortName(),
				e.GetIfaceName(),
			)
		}
	}

	if err := ensureOwnershipAltName(ctx, e); err != nil {
		return err
	}

	if err := moveLink(ctx, src, e.GetIfaceName(), dst, bringUp); err != nil {
		return err
	}

	if err := srcOwner.ReleaseEndpoint(e); err != nil {
		return err
	}
	e.SetNode(dst)
	if err := dstOwner.AdoptEndpoint(e); err != nil {
		e.SetNode(src)
		_ = srcOwner.AdoptEndpoint(e)
		return fmt.Errorf(
			"endpoint %q moved but destination ownership update failed: %w",
			e.GetIfaceName(),
			err,
		)
	}

	return nil
}

func ensureOwnershipAltName(ctx context.Context, e Endpoint) error {
	return e.GetNode().ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(e.GetIfaceName())
		if err != nil {
			return err
		}

		return addOwnershipAltName(link, e)
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

		return fmt.Errorf(
			"interface %s is defined via topology but already exists: %v",
			e.String(),
			err,
		)
	})
}

func moveLink(ctx context.Context, src Node, ifaceName string, dst Node, bringUp bool) error {
	return src.ExecFunction(ctx, func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return err
		}

		if err := netlink.LinkSetDown(link); err != nil {
			return err
		}

		return dst.AddLinkToContainer(ctx, link, func(_ ns.NetNS) error {
			if !bringUp {
				return nil
			}

			movedLink, err := netlink.LinkByName(ifaceName)
			if err != nil {
				return err
			}

			return netlink.LinkSetUp(movedLink)
		})
	})
}

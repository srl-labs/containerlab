package links

import (
	"context"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
)

type genericLinkNode struct {
	shortname string
	endpoints []Endpoint
	nspath    string
}

func newGenericLinkNode(shortname, nspath string) *genericLinkNode {
	return &genericLinkNode{
		shortname: shortname,
		endpoints: []Endpoint{},
		nspath:    nspath,
	}
}

func (g *genericLinkNode) AddLinkToContainer(
	_ context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(g.nspath)
	if err != nil {
		return err
	}
	// move veth endpoint to namespace
	if err := netlink.LinkSetNsFd(link, int(netns.Fd())); err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (g *genericLinkNode) ExecFunction(_ context.Context, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(g.nspath)
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (g *genericLinkNode) AddEndpoint(e Endpoint) error {
	return fmt.Errorf("node %q does not support endpoint registration for %T", g.shortname, e)
}

func (g *genericLinkNode) GetShortName() string {
	return g.shortname
}

func (g *genericLinkNode) GetEndpoints() []Endpoint {
	return g.endpoints
}

func (g *genericLinkNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}

func (g *genericLinkNode) validateEndpointOwner(e Endpoint) error {
	if e.GetNode().GetShortName() == g.shortname {
		return nil
	}

	return fmt.Errorf(
		"endpoint %q is attached to node %q and cannot be added to %q",
		e,
		e.GetNode().GetShortName(),
		g.shortname,
	)
}

func (*genericLinkNode) GetState() clabnodesstate.NodeState {
	// The genericLinkNode is the basis for Mgmt-Bridge and Host fake node.
	// Both of these do generally exist. Hence the Deployed state in generally returned
	return clabnodesstate.Deployed
}

func (g *genericLinkNode) Delete(ctx context.Context) error {
	for _, e := range g.endpoints {
		err := e.GetLink().Remove(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

package links

import (
	"context"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
)

type GenericLinkNode struct {
	shortname string
	endpoints []Endpoint
	nspath    string
}

func (g *GenericLinkNode) AddLinkToContainer(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(g.nspath)
	if err != nil {
		return err
	}
	// move veth endpoint to namespace
	if err = netlink.LinkSetNsFd(link, int(netns.Fd())); err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (g *GenericLinkNode) ExecFunction(_ context.Context, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(g.nspath)
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (g *GenericLinkNode) AddEndpoint(e Endpoint) error {
	g.endpoints = append(g.endpoints, e)

	return nil
}

func (g *GenericLinkNode) GetShortName() string {
	return g.shortname
}

func (g *GenericLinkNode) GetEndpoints() []Endpoint {
	return g.endpoints
}

func (*GenericLinkNode) GetState() state.NodeState {
	// The GenericLinkNode is the basis for Mgmt-Bridge and Host fake node.
	// Both of these do generally exist. Hence the Deployed state in generally returned
	return state.Deployed
}

func (g *GenericLinkNode) Delete(ctx context.Context) error {
	for _, e := range g.endpoints {
		err := e.GetLink().Remove(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

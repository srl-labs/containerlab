package links

import (
	"context"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
)

type ShorthandLinkNode struct {
	shortname    string
	links        []Link
	endpoints    []Endpoint
	nspath       string
	endpointType LinkEndpointType
}

func NewShorthandLinkNode(shortname, nspath string, leType LinkEndpointType) *ShorthandLinkNode {
	return &ShorthandLinkNode{
		shortname:    shortname,
		nspath:       nspath,
		endpointType: leType,
	}
}

func (n *ShorthandLinkNode) AddLinkToContainer(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(n.nspath)
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

func (n *ShorthandLinkNode) ExecFunction(f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(n.nspath)
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (n *ShorthandLinkNode) AddLink(l Link) {
	n.links = append(n.links, l)
}

func (n *ShorthandLinkNode) AddEndpoint(e Endpoint) {
	n.endpoints = append(n.endpoints, e)
}

func (n *ShorthandLinkNode) GetShortName() string {
	return n.shortname
}

func (n *ShorthandLinkNode) GetEndpoints() []Endpoint {
	return n.endpoints
}

func (*ShorthandLinkNode) GetState() state.NodeState {
	// The ShorthandLinkNode is used only in the cmd tools commands
	// so we do assume the referenced nodes are present
	return state.Deployed
}

func (n *ShorthandLinkNode) GetLinkEndpointType() LinkEndpointType {
	return n.endpointType
}

package links

import (
	"context"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
)

// LinkHostRaw is the raw (string) representation of a host link as defined in the topology file.
type LinkHostRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

// ToLinkConfig converts the raw link into a LinkConfig.
func (r *LinkHostRaw) ToLinkConfig() *LinkBrief {
	lc := &LinkBrief{
		Endpoints: make([]string, 2),
		LinkCommonParams: LinkCommonParams{
			MTU:    r.MTU,
			Labels: r.Labels,
			Vars:   r.Vars,
		},
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "host", r.HostInterface)

	return lc
}

func hostLinkFromBrief(lb *LinkBrief, specialEPIndex int) (*LinkHostRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lb, specialEPIndex)

	result := &LinkHostRaw{
		LinkCommonParams: LinkCommonParams{
			MTU:    lb.MTU,
			Labels: lb.Labels,
			Vars:   lb.Vars,
		},
		HostInterface: hostIf,
		Endpoint:      NewEndpointRaw(node, nodeIf, ""),
	}
	return result, nil
}

func (r *LinkHostRaw) GetType() LinkType {
	return LinkTypeHost
}

func (r *LinkHostRaw) Resolve(params *ResolveParams) (Link, error) {
	link := &LinkHost{
		LinkCommonParams: r.LinkCommonParams,
		HostInterface:    r.HostInterface,
	}
	// resolve and populate the endpoint
	ep, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}
	// set the end point in the link
	link.Endpoint = ep
	return link, nil
}

type LinkHost struct {
	LinkCommonParams
	HostInterface string
	Endpoint      Endpoint
}

func (l *LinkHost) Deploy(ctx context.Context) error {
	// build the netlink.Veth struct for the link provisioning
	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.Endpoint.GetRandIfaceName(),
			MTU:  l.MTU,
			// Mac address is set later on
		},
		PeerName: l.HostInterface,
		// PeerMac address is set later on
	}

	// add the link
	err := netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	// add link to node, rename, set mac and Up
	err = l.Endpoint.GetNode().AddNetlinkLinkToContainer(ctx, link, SetNameMACAndUpInterface(link, l.Endpoint))
	if err != nil {
		return err
	}

	// get the link on the host side
	hostLink, err := netlink.LinkByName(l.HostInterface)
	if err != nil {
		return err
	}

	// set the host side link to up
	err = netlink.LinkSetUp(hostLink)
	if err != nil {
		return err
	}

	return nil
}

func (l *LinkHost) GetType() LinkType {
	return LinkTypeHost
}

func (l *LinkHost) Remove(_ context.Context) error {
	// TODO
	return nil
}

func (l *LinkHost) GetEndpoints() []Endpoint {
	return []Endpoint{
		l.Endpoint,
		&EndpointHost{
			EndpointGeneric: EndpointGeneric{
				Node:      GetFakeHostLinkNode(),
				IfaceName: l.HostInterface,
				Link:      l,
			},
		},
	}
}

type GenericLinkNode struct {
	shortname string
	endpoints []Endpoint
	nspath    string
}

func (g *GenericLinkNode) AddNetlinkLinkToContainer(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
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

func (g *GenericLinkNode) ExecFunction(f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	netns, err := ns.GetNS(g.nspath)
	if err != nil {
		return err
	}
	// execute the given function
	return netns.Do(f)
}

func (g *GenericLinkNode) AddLink(l Link) {

}

func (g *GenericLinkNode) AddEndpoint(e Endpoint) {
	g.endpoints = append(g.endpoints, e)
}

func (g *GenericLinkNode) GetShortName() string {
	return g.shortname
}

func (g *GenericLinkNode) GetEndpoints() []Endpoint {
	return g.endpoints
}

func (g *GenericLinkNode) GetState() state.NodeState {
	return state.Unknown
}

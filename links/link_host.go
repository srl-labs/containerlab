package links

import (
	"fmt"
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
	link := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
	}
	// resolve and populate the endpoint
	ep, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}
	hostEp := &EndpointHost{
		EndpointGeneric: EndpointGeneric{
			Node:      GetFakeHostLinkNode(),
			IfaceName: r.HostInterface,
			Link:      link,
		},
	}

	// set the end point in the link
	link.Endpoints = []Endpoint{ep, hostEp}

	return link, nil
}

// type LinkHost struct {
// 	LinkCommonParams
// 	HostEndpoint *EndpointHost
// 	Endpoint     Endpoint
// }

// func (l *LinkHost) Deploy(ctx context.Context) error {
// 	// build the netlink.Veth struct for the link provisioning
// 	link := &netlink.Veth{
// 		LinkAttrs: netlink.LinkAttrs{
// 			Name: l.Endpoint.GetRandIfaceName(),
// 			MTU:  l.MTU,
// 			// Mac address is set later on
// 		},
// 		PeerName: l.HostEndpoint.GetIfaceName(),
// 		// PeerMac address is set later on
// 	}

// 	// add the link
// 	err := netlink.LinkAdd(link)
// 	if err != nil {
// 		return err
// 	}

// 	// add link to node, rename, set mac and Up
// 	err = l.Endpoint.GetNode().AddNetlinkLinkToContainer(ctx, link, SetNameMACAndUpInterface(link, l.Endpoint))
// 	if err != nil {
// 		return err
// 	}

// 	// get the link on the host side
// 	hostLink, err := netlink.LinkByName(l.HostEndpoint.GetIfaceName())
// 	if err != nil {
// 		return err
// 	}

// 	// set the host side link to up
// 	err = netlink.LinkSetUp(hostLink)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (l *LinkHost) GetType() LinkType {
// 	return LinkTypeHost
// }

// func (l *LinkHost) Remove(ctx context.Context) error {
// 	// TODO
// 	return nil
// }

// func (l *LinkHost) GetEndpoints() []Endpoint {
// 	return []Endpoint{
// 		l.Endpoint,
// 		l.HostEndpoint,
// 	}
// }

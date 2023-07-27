package types

import (
	"context"
	"fmt"

	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type LinkMgmtNetRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *LinkMgmtNetRaw) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: make([]string, 2),
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "mgmt-net", r.HostInterface)

	return lc
}

func (r *LinkMgmtNetRaw) Resolve(params *ResolveParams) (LinkInterf, error) {

	// create the LinkMgmtNet struct
	link := &LinkMgmtNet{
		LinkCommonParams: r.LinkCommonParams,
	}

	bridgeEp := &EndptGeneric{
		Node:  GetFakeMgmtBrLinkNode(),
		state: EndptDeployStateDeployed,
		Iface: r.HostInterface,
		Link:  link,
	}

	link.BridgeEndpoint = bridgeEp

	// resolve and populate the endpoint
	ep, err := r.Endpoint.Resolve(params.Nodes, link)
	if err != nil {
		return nil, err
	}
	// set the end point in the link
	link.ContainerEndpoint = ep
	return link, nil
}

func (r *LinkMgmtNetRaw) GetType() LinkType {
	return LinkTypeMgmtNet
}

func mgmtNetFromLinkConfig(lc LinkConfig, specialEPIndex int) (*LinkMgmtNetRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &LinkMgmtNetRaw{
		LinkCommonParams: LinkCommonParams{
			Mtu:    lc.MTU,
			Labels: lc.Labels,
			Vars:   lc.Vars,
		},
		HostInterface: hostIf,
		Endpoint:      NewEndpointRaw(node, nodeIf, ""),
	}
	return result, nil
}

type LinkMgmtNet struct {
	LinkCommonParams
	BridgeEndpoint    Endpt
	ContainerEndpoint Endpt
}

func (*LinkMgmtNet) GetType() LinkType {
	return LinkTypeMgmtNet
}

func (l *LinkMgmtNet) Deploy(ctx context.Context) error {
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.ContainerEndpoint.GetRandIfaceName(),
			MTU:  l.Mtu,
		},
		PeerName: l.BridgeEndpoint.GetIfaceName(),
	}
	err := netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// add link to node, rename, set mac and Up
	err = l.ContainerEndpoint.GetNode().AddNetlinkLinkToContainer(ctx, linkA, SetNameMACAndUpInterface(linkA, l.ContainerEndpoint))
	if err != nil {
		return err
	}

	// get host side interface
	linkB, err := netlink.LinkByName(l.BridgeEndpoint.GetIfaceName())
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", l.BridgeEndpoint.GetIfaceName(), err)
	}

	// retrieve the bridge
	br, err := utils.BridgeByName(l.BridgeEndpoint.GetNode().GetShortName())
	if err != nil {
		return err
	}

	// connect host veth end to the bridge
	if err := netlink.LinkSetMaster(linkB, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", l.BridgeEndpoint.GetIfaceName(), l.BridgeEndpoint.GetNode().GetShortName(), err)
	}

	// set the host side interface, attached to the bridge, to up
	if err = netlink.LinkSetUp(linkB); err != nil {
		return fmt.Errorf("failed to set %q up: %v", l.BridgeEndpoint.GetIfaceName(), err)
	}
	return nil
}

func (l *LinkMgmtNet) Remove(ctx context.Context) error {
	// TODO
	return nil
}

func (l *LinkMgmtNet) GetEndpoints() []Endpt {
	return []Endpt{
		l.ContainerEndpoint,
		l.BridgeEndpoint,
	}
}

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
	MgmtBridge       string       `yaml:"bridge"`
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

func (r *LinkMgmtNetRaw) Resolve(nodes map[string]LinkNode) (LinkInterf, error) {
	// create the LinkMgmtNet struct
	link := &LinkMgmtNet{
		LinkCommonParams: r.LinkCommonParams,
		HostInterface:    r.HostInterface,
		MgmtBridge:       r.MgmtBridge,
	}

	// resolve and populate the endpoint
	ep, err := r.Endpoint.Resolve(nodes)
	if err != nil {
		return nil, err
	}
	// set the end point in the link
	link.ContainerEndpoint = ep
	return link, nil
}

func mgmtNetFromLinkConfig(lc LinkConfig, specialEPIndex int) (RawLink, error) {
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
	HostInterface     string
	MgmtBridge        string
	ContainerEndpoint *Endpt
}

func (*LinkMgmtNet) GetType() LinkType {
	return LinkTypeVEth
}

func (l *LinkMgmtNet) Deploy(ctx context.Context) error {
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.ContainerEndpoint.GetRandName(),
			MTU:  l.Mtu,
		},
		PeerName: l.HostInterface,
	}
	err := netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// add link to node, rename, set mac and Up
	err = l.ContainerEndpoint.Node.AddLink(ctx, linkA, SetNameMACAndUpInterface(linkA, l.ContainerEndpoint))
	if err != nil {
		return err
	}

	// get host side interface
	linkB, err := netlink.LinkByName(l.HostInterface)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", l.HostInterface, err)
	}

	// retrieve the bridge
	br, err := utils.BridgeByName(l.MgmtBridge)
	if err != nil {
		return err
	}

	// connect host veth end to the bridge
	if err := netlink.LinkSetMaster(linkB, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", l.HostInterface, l.MgmtBridge, err)
	}

	// set the host side interface, attached to the bridge, to up
	if err = netlink.LinkSetUp(linkB); err != nil {
		return fmt.Errorf("failed to set %q up: %v", l.HostInterface, err)
	}
	return nil
}

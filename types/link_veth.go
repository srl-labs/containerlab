package types

import (
	"context"
	"fmt"
	"sync"

	"github.com/vishvananda/netlink"
)

// LinkVEthRaw is the raw (string) representation of a veth link as defined in the topology file.
type LinkVEthRaw struct {
	LinkCommonParams `yaml:",inline"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

func (r *LinkVEthRaw) MarshalYAML() (interface{}, error) {
	x := struct {
		Type        string `yaml:"type"`
		LinkVEthRaw `yaml:",inline"`
	}{
		Type:        string(LinkTypeVEth),
		LinkVEthRaw: *r,
	}
	return x, nil
}

// ToLinkConfig converts the raw link into a LinkConfig.
func (r *LinkVEthRaw) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: []string{},
	}
	for _, e := range r.Endpoints {
		lc.Endpoints = append(lc.Endpoints, fmt.Sprintf("%s:%s", e.Node, e.Iface))
	}
	return lc
}

func (r *LinkVEthRaw) GetType() LinkType {
	return LinkTypeVEth
}

func (r *LinkVEthRaw) Resolve(params *ResolveParams) (LinkInterf, error) {

	// create LinkVEth struct
	l := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
		Endpoints:        make([]*EndptGeneric, 0, 2),
	}

	// resolve endpoints
	for _, ep := range r.Endpoints {
		// resolve endpoint
		ept, err := ep.Resolve(params.Nodes, l)
		if err != nil {
			return nil, err
		}
		// set resolved endpoint in link endpoints
		l.Endpoints = append(l.Endpoints, ept)
	}

	return l, nil
}

func vEthFromLinkConfig(lc LinkConfig) (*LinkVEthRaw, error) {
	host, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, 0)

	result := &LinkVEthRaw{
		LinkCommonParams: LinkCommonParams{
			Mtu:    lc.MTU,
			Labels: lc.Labels,
			Vars:   lc.Vars,
		},
		Endpoints: []*EndpointRaw{
			NewEndpointRaw(host, hostIf, ""),
			NewEndpointRaw(node, nodeIf, ""),
		},
	}
	return result, nil
}

type LinkVEth struct {
	// m mutex is used when deployign the link.
	m sync.Mutex `yaml:"-"`
	LinkCommonParams
	Endpoints []*EndptGeneric
}

func (*LinkVEth) GetType() LinkType {
	return LinkTypeVEth
}

func (l *LinkVEth) Deploy(ctx context.Context) error {
	l.m.Lock()
	defer l.m.Unlock()

	for _, ep := range l.Endpoints {
		if ep.state != EndptDeployStateReady {
			return nil
		}
	}

	// build the netlink.Veth struct for the link provisioning
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.Endpoints[0].GetRandIfaceName(),
			MTU:  l.Mtu,
			// Mac address is set later on
		},
		PeerName: l.Endpoints[1].GetRandIfaceName(),
		// PeerMac address is set later on
	}

	// add the link
	err := netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// retrieve the netlink.Link for the B / Peer side of the link
	linkB, err := netlink.LinkByName(l.Endpoints[1].GetRandIfaceName())
	if err != nil {
		return err
	}

	// for both ends of the link
	for idx, link := range []netlink.Link{linkA, linkB} {

		switch l.Endpoints[idx].GetNode().GetLinkEndpointType() {
		case LinkEndpointTypeBridge:
			// if the node is a bridge kind (linux bridge or ovs-bridge)
			// retrieve bridge name via node name
			bridgeName := l.Endpoints[idx].GetNode().GetShortName()

			// retrieve the bridg link
			bridge, err := netlink.LinkByName(bridgeName)
			if err != nil {
				return err
			}

			// set the retrieved bridge as the master for the actual link
			err = netlink.LinkSetMaster(link, bridge)
			if err != nil {
				return err
			}
		default:
			// if the node is a regular namespace node
			// add link to node, rename, set mac and Up
			err = l.Endpoints[idx].GetNode().AddNetlinkLinkToContainer(ctx, link, SetNameMACAndUpInterface(link, l.Endpoints[idx]))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *LinkVEth) Remove(_ context.Context) error {
	// TODO
	return nil
}

func (l *LinkVEth) GetEndpoints() []Endpt {
	result := make([]Endpt, 0, len(l.Endpoints))
	for i, e := range l.Endpoints {
		result[i] = e
	}
	return result

}

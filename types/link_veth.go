package types

import (
	"context"
	"fmt"

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

func (r *LinkVEthRaw) Resolve(nodes map[string]LinkNode) (LinkInterf, error) {

	// create LinkVEth struct
	l := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
		Endpoints:        make([]*Endpt, 0, 2),
	}

	// resolve endpoints
	for idx, ep := range r.Endpoints {
		// resolve endpoint
		ept, err := ep.Resolve(nodes)
		if err != nil {
			return nil, err
		}
		// set resolved endpoint in link endpoints
		l.Endpoints[idx] = ept
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
	LinkCommonParams
	Endpoints []*Endpt
}

func (*LinkVEth) GetType() LinkType {
	return LinkTypeVEth
}

func (l *LinkVEth) Deploy(ctx context.Context) error {
	// build the netlink.Veth struct for the link provisioning
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.Endpoints[0].GetRandName(),
			MTU:  l.Mtu,
			// Mac address is set later on
		},
		PeerName: l.Endpoints[1].GetRandName(),
		// PeerMac address is set later on
	}

	// add the link
	err := netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// retrieve the netlink.Link for the B / Peer side of the link
	linkB, err := netlink.LinkByName(l.Endpoints[1].GetRandName())
	if err != nil {
		return err
	}

	// for both ends of the link
	for idx, link := range []netlink.Link{linkA, linkB} {
		// add link to node, rename, set mac and Up
		err = l.Endpoints[idx].Node.AddLink(ctx, link, SetNameMACAndUpInterface(link, l.Endpoints[idx]))
		if err != nil {
			return err
		}
	}

	return nil
}

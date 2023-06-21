package links

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type RawVEthLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	Mtu              int            `yaml:"mtu,omitempty"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

func (r *RawVEthLink) UnRaw(res Resolver) (Link, error) {
	result := &VEthLink{
		Endpoints: make([]*Endpoint, len(r.Endpoints)),
		LinkGenericAttrs: LinkGenericAttrs{
			Labels: r.Labels,
			Vars:   r.Vars,
		},
		Mtu: r.Mtu,
	}

	var err error
	for idx, e := range r.Endpoints {
		result.Endpoints[idx], err = e.UnRaw(res)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func vEthFromLinkConfig(lc LinkConfig) (*RawVEthLink, error) {
	nodeA, nodeAIf, nodeB, nodeBIf := extractHostNodeInterfaceData(lc, 0)

	result := &RawVEthLink{
		RawLinkTypeAlias: RawLinkTypeAlias{
			Type:     string(LinkTypeVEth),
			Labels:   lc.Labels,
			Vars:     lc.Vars,
			Instance: nil,
		},
		Mtu: lc.MTU,
		Endpoints: []*EndpointRaw{
			{
				Node:  nodeA,
				Iface: nodeAIf,
			},
			{
				Node:  nodeB,
				Iface: nodeBIf,
			},
		},
	}
	return result, nil
}

type VEthLink struct {
	LinkGenericAttrs
	Mtu       int
	Endpoints []*Endpoint
}

func (m *VEthLink) GetType() (LinkType, error) {
	return LinkTypeVEth, nil
}

func (m *VEthLink) Deploy() error {
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:         m.Endpoints[0].GetRandName(),
			HardwareAddr: m.Endpoints[0].MacAddress,
			Flags:        net.FlagUp,
			MTU:          m.Mtu,
		},
		PeerName:         m.Endpoints[1].GetRandName(),
		PeerHardwareAddr: m.Endpoints[1].MacAddress,
	}

	if err := netlink.LinkAdd(linkA); err != nil {
		return err
	}
	linkB, err := netlink.LinkByName(m.Endpoints[1].GetRandName())
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", m.Endpoints[1].GetRandName(), err)
	}

	// push interfaces to namespaces and rename to final interface names
	links := []netlink.Link{linkA, linkB}
	for idx, endpoint := range m.Endpoints {
		err := toNS(links[idx], endpoint.Node.Config().NSPath, endpoint.Iface)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *VEthLink) Remove() error {
	// TODO
	log.Warn("not implemented yet")
	return nil
}

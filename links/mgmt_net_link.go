package links

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
)

type RawMgmtNetLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (m *RawMgmtNetLink) UnRaw(res Resolver) (Link, error) {
	n, err := res.ResolveNode(m.Endpoint.Node)
	if err != nil {
		return nil, err
	}
	mac, err := net.ParseMAC(m.Endpoint.Mac)
	if err != nil {
		return nil, err
	}
	e := NewEndpoint(n, m.Endpoint.Iface, mac)
	return &MgmtNetLink{
		LinkGenericAttrs: LinkGenericAttrs{
			Labels: m.Labels,
			Vars:   m.Vars,
		},
		HostInterface:     m.HostInterface,
		ContainerEndpoint: e,
	}, nil
}

func mgmtNetFromLinkConfig(lc LinkConfig, specialEPIndex int) (*RawMgmtNetLink, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	e :=&

	result := &RawMgmtNetLink{
		RawLinkTypeAlias: RawLinkTypeAlias{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
		HostInterface:    hostIf,
		Node:             node,
		NodeInterface:    nodeIf,
	}
	return result, nil
}

type MgmtNetLink struct {
	LinkGenericAttrs
	HostInterface     string
	ContainerEndpoint *Endpoint
}

func (m *MgmtNetLink) Deploy() error {
	// TODO
	return fmt.Errorf("not yet implemented")
}

func (m *MgmtNetLink) GetType() (LinkType, error) {
	return LinkTypeMgmtNet, nil
}

func (m *MgmtNetLink) Remove() error {
	// TODO
	log.Warn("not implemented yet")
	return nil

}

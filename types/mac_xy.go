package types

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type rawMacVXType struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
	MAC              string `yaml:"mac"`
}

// convert the rawMacVXType to macVXType
func (r *rawMacVXType) UnRaw(res NodeResolver) (*macVXType, error) {
	node, err := res.ResolveNode(r.Node)
	if err != nil {
		return nil, err
	}

	cEndpoint := NewEndpoint(node, r.NodeInterface, net.HardwareAddr(r.MAC))

	return &macVXType{
		HostInterface:     r.HostInterface,
		ContainerEndpoint: cEndpoint,
		LinkStatus:        LinkStatus{},
	}, nil
}

func macVXTypeFromLinkConfig(lc LinkConfig, specialEPIndex int) (*rawMacVXType, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &rawMacVXType{
		RawLinkTypeAlias: RawLinkTypeAlias{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
		HostInterface:    hostIf,
		Node:             node,
		NodeInterface:    nodeIf,
	}
	return result, nil
}

type macVXType struct {
	HostInterface     string
	ContainerEndpoint *Endpoint
	MAC               string
	LinkGenericAttrs
	LinkStatus
}

func (m *macVXType) Deploy(iftype LinkType) error {
	parentInterface, err := netlink.LinkByName(m.HostInterface)
	if err != nil {
		return err
	}

	mvl := netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        m.ContainerEndpoint.GetRandName(),
			ParentIndex: parentInterface.Attrs().Index,
		},
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}

	var link netlink.Link
	switch iftype {
	case LinkTypeMacVTap:
		link = &netlink.Macvtap{Macvlan: mvl}
	case LinkTypeMacVLan:
		link = &mvl
	}

	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	var mvInterface netlink.Link
	if mvInterface, err = netlink.LinkByName(m.ContainerEndpoint.GetRandName()); err != nil {
		return fmt.Errorf("failed to lookup %q: %v", m.ContainerEndpoint.GetRandName(), err)
	}

	err = toNS(mvInterface, m.ContainerEndpoint.Node.GetNamespacePath(), m.ContainerEndpoint.Iface)
	if err != nil {
		return err
	}

	err = netlink.LinkSetHardwareAddr(mvInterface, net.HardwareAddr(m.MAC))
	if err != nil {
		return err
	}

	return nil
}

func (m *macVXType) Remove(lt LinkType) error {
	// TODO
	log.Warn("not implemented yet")
	return nil
}

func (m *macVXType) GetEndpoints() []*Endpoint {
	return []*Endpoint{m.ContainerEndpoint}
}

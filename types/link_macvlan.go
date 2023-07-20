package types

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
)

// LinkMacVlanRaw is the raw (string) representation of a macvlan link as defined in the topology file.
type LinkMacVlanRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
	Mode             string       `yaml:"mode"`
}

// ToLinkConfig converts the raw link into a LinkConfig.
func (r *LinkMacVlanRaw) ToLinkConfig() *LinkConfig {
	lc := &LinkConfig{
		Vars:      r.Vars,
		Labels:    r.Labels,
		MTU:       r.Mtu,
		Endpoints: make([]string, 2),
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "macvlan", r.HostInterface)

	return lc
}

func macVlanFromLinkConfig(lc LinkConfig, specialEPIndex int) (*LinkMacVlanRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &LinkMacVlanRaw{
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

func (r *LinkMacVlanRaw) Resolve(nodes map[string]LinkNode) (LinkInterf, error) {

	link := &LinkMacVlan{
		LinkCommonParams: r.LinkCommonParams,
		HostInterface:    r.HostInterface,
	}
	// parse the MacVlanMode
	mode, err := MacVlanModeParse(r.Mode)
	if err != nil {
		return nil, err
	}
	// set the mode in the link struct
	link.Mode = mode
	// resolve the endpoint
	ep, err := r.Endpoint.Resolve(nodes)
	if err != nil {
		return nil, err
	}
	// set the indpoint in the link struct
	link.NodeEndpoint = ep

	return link, nil
}

type LinkMacVlan struct {
	LinkCommonParams
	HostInterface string
	NodeEndpoint  *Endpt
	Mode          MacVlanMode
}

type MacVlanMode string

const (
	MacVlanModeBridge   = "bridge"
	MacVlanModeVepa     = "vepa"
	MacVlanModePassthru = "passthru"
	MacVlanModePrivate  = "private"
	MacVlanModeSource   = "source"
)

func MacVlanModeParse(s string) (MacVlanMode, error) {
	switch s {
	case MacVlanModeBridge:
		return MacVlanModeBridge, nil
	case MacVlanModeVepa:
		return MacVlanModeVepa, nil
	case MacVlanModePassthru:
		return MacVlanModePassthru, nil
	case MacVlanModePrivate:
		return MacVlanModePrivate, nil
	case MacVlanModeSource:
		return MacVlanModeSource, nil
	case "":
		return MacVlanModeBridge, nil
	}
	return "", fmt.Errorf("unknown MacVlanMode %q", s)
}

func (l *LinkMacVlan) GetType() LinkType {
	return LinkTypeMacVLan
}

func (l *LinkMacVlan) Deploy(ctx context.Context) error {
	// lookup the parent host interface
	parentInterface, err := netlink.LinkByName(l.HostInterface)
	if err != nil {
		return err
	}

	// set MacVlanMode
	mode := netlink.MACVLAN_MODE_BRIDGE
	switch l.Mode {
	case MacVlanModeBridge:
		break
	case MacVlanModePassthru:
		mode = netlink.MACVLAN_MODE_PASSTHRU
	case MacVlanModeVepa:
		mode = netlink.MACVLAN_MODE_VEPA
	case MacVlanModePrivate:
		mode = netlink.MACVLAN_MODE_PRIVATE
	case MacVlanModeSource:
		mode = netlink.MACVLAN_MODE_SOURCE
	}

	// build Netlink Macvlan struct
	link := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        l.NodeEndpoint.GetRandName(),
			ParentIndex: parentInterface.Attrs().Index,
			MTU:         l.Mtu,
		},
		Mode: mode,
	}
	// add the link in the Host NetNS
	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	// retrieve the Link by name
	mvInterface, err := netlink.LinkByName(l.NodeEndpoint.GetRandName())
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", l.NodeEndpoint.GetRandName(), err)
	}

	// add the link to the Node Namespace
	err = l.NodeEndpoint.Node.AddLink(ctx, mvInterface, SetNameMACAndUpInterface(mvInterface, l.NodeEndpoint))
	return err
}

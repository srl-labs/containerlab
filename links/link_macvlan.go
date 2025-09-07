package links

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/vishvananda/netlink"
)

// LinkMacVlanRaw is the raw (string) representation of a macvlan link as defined in the topology file.
type LinkMacVlanRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
	Mode             string       `yaml:"mode"`
}

// ToLinkBriefRaw converts the raw link into a LinkConfig.
func (r *LinkMacVlanRaw) ToLinkBriefRaw() *LinkBriefRaw {
	lc := &LinkBriefRaw{
		Endpoints: make([]string, 2),
		LinkCommonParams: LinkCommonParams{
			MTU:    r.MTU,
			Labels: r.Labels,
			Vars:   r.Vars,
		},
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "macvlan", r.HostInterface)

	return lc
}

func (*LinkMacVlanRaw) GetType() LinkType {
	return LinkTypeMacVLan
}

func macVlanLinkFromBrief(lb *LinkBriefRaw, specialEPIndex int) (*LinkMacVlanRaw, error) {
	_, hostIf, node, nodeIf, err := extractHostNodeInterfaceData(lb, specialEPIndex)
	if err != nil {
		return nil, err
	}
	link := &LinkMacVlanRaw{
		LinkCommonParams: lb.LinkCommonParams,
		HostInterface:    hostIf,
		Endpoint:         NewEndpointRaw(node, nodeIf, ""),
	}

	// set default link mtu if MTU is unset
	if link.MTU == 0 {
		link.MTU = clabconstants.DefaultLinkMTU
	}

	return link, nil
}

func (r *LinkMacVlanRaw) Resolve(params *ResolveParams) (Link, error) {
	var err error
	// filtered true means the link is in the filter provided by a user
	// aka it should be resolved/created/deployed
	filtered := isInFilter(params, []*EndpointRaw{r.Endpoint})
	if !filtered {
		return nil, nil
	}

	// create the MacVlan Link
	link := &LinkMacVlan{
		LinkCommonParams: r.LinkCommonParams,
	}
	// create the host side MacVlan Endpoint
	link.HostEndpoint = &EndpointMacVlan{
		EndpointGeneric: *NewEndpointGeneric(GetHostLinkNode(), r.HostInterface, link),
	}

	// populate the host interfaces mac address
	hostLink, err := netlink.LinkByName(r.HostInterface)
	if err != nil {
		return nil, err
	}
	link.HostEndpoint.MAC = hostLink.Attrs().HardwareAddr

	// parse the MacVlanMode
	mode, err := MacVlanModeParse(r.Mode)
	if err != nil {
		return nil, err
	}
	// set the mode in the link struct
	link.Mode = mode
	// resolve the endpoint
	link.NodeEndpoint, err = r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}

	// propagate the parent interface MTU to the link
	// because the macvlan interface MTU is inherited from
	// its parent interface
	link.MTU, err = link.GetParentInterfaceMTU()
	if err != nil {
		return nil, err
	}

	return link, nil
}

type LinkMacVlan struct {
	LinkCommonParams
	HostEndpoint *EndpointMacVlan
	NodeEndpoint Endpoint
	Mode         MacVlanMode
}

func (*LinkMacVlan) GetType() LinkType {
	return LinkTypeMacVLan
}

func (l *LinkMacVlan) GetParentInterfaceMTU() (int, error) {
	hostLink, err := netlink.LinkByName(l.HostEndpoint.GetIfaceName())
	if err != nil {
		return 0, err
	}
	return hostLink.Attrs().MTU, nil
}

func (l *LinkMacVlan) Deploy(ctx context.Context, _ Endpoint) error {
	// lookup the parent host interface
	parentInterface, err := netlink.LinkByName(l.HostEndpoint.GetIfaceName())
	if err != nil {
		return err
	}

	log.Infof("Creating MACVLAN link: %s <--> %s", l.HostEndpoint, l.NodeEndpoint)

	// build Netlink Macvlan struct
	link := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        l.NodeEndpoint.GetRandIfaceName(),
			ParentIndex: parentInterface.Attrs().Index,
		},
		Mode: l.Mode.ToNetlinkMode(),
	}
	// add the link in the Host NetNS
	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	// retrieve the Link by name
	mvInterface, err := netlink.LinkByName(l.NodeEndpoint.GetRandIfaceName())
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", l.NodeEndpoint.GetRandIfaceName(), err)
	}

	// enable promiscuous mode
	err = netlink.SetPromiscOn(mvInterface)
	if err != nil {
		return fmt.Errorf("failed setting promiscuous mode for interface %s (%s:%s): %v",
			l.NodeEndpoint.GetRandIfaceName(), l.NodeEndpoint.GetNode().GetShortName(), l.NodeEndpoint.GetIfaceName(), err)
	}

	// add the link to the Node Namespace
	err = l.NodeEndpoint.GetNode().AddLinkToContainer(ctx, mvInterface,
		SetNameMACAndUpInterface(mvInterface, l.NodeEndpoint))
	return err
}

func (l *LinkMacVlan) Remove(ctx context.Context) error {
	// check Deployment state, if the Link was already
	// removed via e.g. the peer node
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	// trigger link removal via the NodeEndpoint
	err := l.NodeEndpoint.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// adjust the Deployment status to reflect the removal
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

func (l *LinkMacVlan) GetEndpoints() []Endpoint {
	return []Endpoint{
		l.NodeEndpoint,
		l.HostEndpoint,
	}
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

func (m MacVlanMode) ToNetlinkMode() netlink.MacvlanMode {
	var mode netlink.MacvlanMode
	switch m {
	case MacVlanModeBridge:
		mode = netlink.MACVLAN_MODE_BRIDGE
	case MacVlanModePassthru:
		mode = netlink.MACVLAN_MODE_PASSTHRU
	case MacVlanModeVepa:
		mode = netlink.MACVLAN_MODE_VEPA
	case MacVlanModePrivate:
		mode = netlink.MACVLAN_MODE_PRIVATE
	case MacVlanModeSource:
		mode = netlink.MACVLAN_MODE_SOURCE
	}
	return mode
}

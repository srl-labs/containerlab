package links

import (
	"context"
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type LinkMgmtNetRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *LinkMgmtNetRaw) ToLinkBriefRaw() *LinkBriefRaw {
	lc := &LinkBriefRaw{
		Endpoints: make([]string, 2),
		LinkCommonParams: LinkCommonParams{
			MTU:    r.MTU,
			Labels: r.Labels,
			Vars:   r.Vars,
		},
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "mgmt-net", r.HostInterface)

	return lc
}

func (r *LinkMgmtNetRaw) Resolve(params *ResolveParams) (Link, error) {
	// filtered true means the link is in the filter provided by a user
	// aka it should be resolved/created/deployed
	filtered := isInFilter(params, []*EndpointRaw{r.Endpoint})
	if !filtered {
		return nil, nil
	}

	// create the LinkMgmtNet struct
	link := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
	}

	mgmtBridgeNode := GetMgmtBrLinkNode()

	bridgeEp := NewEndpointBridge(NewEndpointGeneric(mgmtBridgeNode, r.HostInterface, link), true)

	var err error
	bridgeEp.MAC, err = utils.GenMac(ClabOUI)
	if err != nil {
		return nil, err
	}

	// resolve and populate the endpoint
	contEp, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}

	link.Endpoints = []Endpoint{bridgeEp, contEp}

	// add link to respective endpoint nodes
	_ = bridgeEp.GetNode().AddEndpoint(bridgeEp)

	// set default link mtu if MTU is unset
	if link.MTU == 0 {
		link.MTU = DefaultLinkMTU
	}

	return link, nil
}

func (*LinkMgmtNetRaw) GetType() LinkType {
	return LinkTypeMgmtNet
}

func mgmtNetLinkFromBrief(lb *LinkBriefRaw, specialEPIndex int) (*LinkMgmtNetRaw, error) {
	_, hostIf, node, nodeIf, err := extractHostNodeInterfaceData(lb, specialEPIndex)
	if err != nil {
		return nil, err
	}

	link := &LinkMgmtNetRaw{
		LinkCommonParams: lb.LinkCommonParams,
		HostInterface:    hostIf,
		Endpoint:         NewEndpointRaw(node, nodeIf, ""),
	}

	// set default link mtu if MTU is unset
	if link.MTU == 0 {
		link.MTU = DefaultLinkMTU
	}

	return link, nil
}

var _mgmtBrLinkMgmtBrInstance *mgmtBridgeLinkNode

// mgmtBridgeLinkNode is a special node that represents the mgmt bridge node
// that is used when mgmt-net link is defined in the topology.
type mgmtBridgeLinkNode struct {
	GenericLinkNode
}

func (*mgmtBridgeLinkNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeBridge
}

func (b *mgmtBridgeLinkNode) AddLinkToContainer(_ context.Context, link netlink.Link, f func(ns.NetNS) error) error {
	// retrieve the namespace handle
	ns, err := ns.GetCurrentNS()
	if err != nil {
		return err
	}

	// get the bridge as netlink.Link
	br, err := netlink.LinkByName(b.shortname)
	if err != nil {
		return err
	}

	// assign the bridge to the link as master
	err = netlink.LinkSetMaster(link, br)
	if err != nil {
		return err
	}

	// execute the given function
	return ns.Do(f)
}

func getMgmtBrLinkNode() *mgmtBridgeLinkNode {
	if _mgmtBrLinkMgmtBrInstance == nil {
		currns, err := ns.GetCurrentNS()
		if err != nil {
			log.Error(err)
		}
		nspath := currns.Path()
		_mgmtBrLinkMgmtBrInstance = &mgmtBridgeLinkNode{
			GenericLinkNode: GenericLinkNode{
				shortname: "mgmt-net",
				endpoints: []Endpoint{},
				nspath:    nspath,
			},
		}
	}
	return _mgmtBrLinkMgmtBrInstance
}

func GetMgmtBrLinkNode() Node { // skipcq: RVV-B0001
	return getMgmtBrLinkNode()
}

func SetMgmtNetUnderlayingBridge(bridge string) error {
	getMgmtBrLinkNode().GenericLinkNode.shortname = bridge
	return nil
}

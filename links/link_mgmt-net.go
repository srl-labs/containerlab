package links

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type LinkMgmtNetRaw struct {
	LinkCommonParams `             yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
	// Network is the name of the management network the link attaches to.
	// An empty value selects the default (first) management network.
	Network string `yaml:"network,omitempty"`
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

	// Normalize link vars to ensure JSON serialization compatibility
	link.Vars = normalizeVars(link.Vars)

	network := r.Network
	if network == params.DefaultMgmtNetwork {
		network = ""
	}
	mgmtBridgeNode := getMgmtBrLinkNode(network)
	if mgmtBridgeNode == nil {
		return nil, fmt.Errorf("failed to get the mgmt-net link node for network %q", r.Network)
	}

	bridgeEp := NewEndpointBridge(NewEndpointGeneric(mgmtBridgeNode, r.HostInterface, link), true)

	var err error
	bridgeEp.MAC, err = clabutils.GenMac(clabconstants.ClabOUI)
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
		link.MTU = clabconstants.DefaultLinkMTU
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
		link.MTU = clabconstants.DefaultLinkMTU
	}

	return link, nil
}

// mgmtNetOwnerName is the logical owner name of the interfaces created for
// mgmt-net links, shared by the mgmt-net link nodes of all management networks.
const mgmtNetOwnerName = "mgmt-net"

var (
	// _mgmtBrLinkNodes holds the mgmt-net link nodes keyed by the management
	// network name. The empty name is the default management network.
	_mgmtBrLinkNodes   = map[string]*mgmtBridgeLinkNode{}
	_mgmtBrLinkNodesMu sync.Mutex
)

// mgmtBridgeLinkNode is a special node that represents the mgmt bridge node
// that is used when mgmt-net link is defined in the topology.
type mgmtBridgeLinkNode struct {
	GenericLinkNode
}

func (*mgmtBridgeLinkNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeBridge
}

func (b *mgmtBridgeLinkNode) AddLinkToContainer(
	_ context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	// retrieve the namespace handle
	curNamespace, err := ns.GetCurrentNS()
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
	return curNamespace.Do(f)
}

// getMgmtBrLinkNode returns the mgmt-net link node of the given management network,
// creating it on first use. The empty network name selects the default network.
func getMgmtBrLinkNode(network string) *mgmtBridgeLinkNode {
	_mgmtBrLinkNodesMu.Lock()
	defer _mgmtBrLinkNodesMu.Unlock()

	if n, ok := _mgmtBrLinkNodes[network]; ok {
		return n
	}

	currns, err := ns.GetCurrentNS()
	if err != nil {
		log.Errorf("failed to get current network namespace: %v", err)
		return nil
	}
	n := &mgmtBridgeLinkNode{
		GenericLinkNode: GenericLinkNode{
			shortname: mgmtNetOwnerName,
			endpoints: []Endpoint{},
			nspath:    currns.Path(),
		},
	}
	_mgmtBrLinkNodes[network] = n

	return n
}

// GetMgmtBrLinkNode returns the mgmt-net link node of the default management network.
func GetMgmtBrLinkNode() Node { // skipcq: RVV-B0001
	return getMgmtBrLinkNode("")
}

// GetMgmtBrLinkNodes returns the mgmt-net link nodes of all management networks
// that have been referenced so far, starting with the default network.
func GetMgmtBrLinkNodes() []Node {
	var nodes []Node
	if n := getMgmtBrLinkNode(""); n != nil {
		nodes = append(nodes, n)
	}

	_mgmtBrLinkNodesMu.Lock()
	defer _mgmtBrLinkNodesMu.Unlock()

	networks := make([]string, 0, len(_mgmtBrLinkNodes))
	for network := range _mgmtBrLinkNodes {
		if network != "" {
			networks = append(networks, network)
		}
	}
	slices.Sort(networks)

	for _, network := range networks {
		nodes = append(nodes, _mgmtBrLinkNodes[network])
	}

	return nodes
}

// SetMgmtNetUnderlyingBridge sets the bridge of the default management network.
func SetMgmtNetUnderlyingBridge(bridge string) error {
	return SetMgmtNetworkUnderlyingBridge("", bridge)
}

// SetMgmtNetworkUnderlyingBridge sets the bridge the mgmt-net links of the given
// management network attach to. The empty network name selects the default network.
func SetMgmtNetworkUnderlyingBridge(network, bridge string) error {
	n := getMgmtBrLinkNode(network)
	if n == nil {
		return fmt.Errorf("failed to get the mgmt-net link node for network %q", network)
	}
	n.GenericLinkNode.shortname = bridge
	return nil
}

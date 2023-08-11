package links

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

type LinkMgmtNetRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *LinkMgmtNetRaw) ToLinkBrief() *LinkBriefRaw {
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

	// create the LinkMgmtNet struct
	link := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
	}

	fakeMgmtBridgeNode := GetFakeMgmtBrLinkNode()

	bridgeEp := &EndpointBridge{
		EndpointGeneric: *NewEndpointGeneric(fakeMgmtBridgeNode, r.HostInterface),
	}
	bridgeEp.Link = link

	var err error
	bridgeEp.MAC, err = utils.GenMac(ClabOUI)
	if err != nil {
		return nil, err
	}

	// add endpoint to fake mgmt bridge node
	fakeMgmtBridgeNode.AddEndpoint(bridgeEp)

	// resolve and populate the endpoint
	contEp, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}

	link.Endpoints = []Endpoint{bridgeEp, contEp}

	return link, nil
}

func (r *LinkMgmtNetRaw) GetType() LinkType {
	return LinkTypeMgmtNet
}

func mgmtNetLinkFromBrief(lb *LinkBriefRaw, specialEPIndex int) (*LinkMgmtNetRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lb, specialEPIndex)

	result := &LinkMgmtNetRaw{
		LinkCommonParams: LinkCommonParams{
			MTU:    lb.MTU,
			Labels: lb.Labels,
			Vars:   lb.Vars,
		},
		HostInterface: hostIf,
		Endpoint:      NewEndpointRaw(node, nodeIf, ""),
	}
	return result, nil
}

var _fakeMgmtBrLinkMgmtBrInstance *fakeMgmtBridgeLinkNode

type fakeMgmtBridgeLinkNode struct {
	GenericLinkNode
}

func (*fakeMgmtBridgeLinkNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeBridge
}

func getFakeMgmtBrLinkNode() *fakeMgmtBridgeLinkNode {
	if _fakeMgmtBrLinkMgmtBrInstance == nil {
		currns, err := ns.GetCurrentNS()
		if err != nil {
			log.Error(err)
		}
		nspath := currns.Path()
		_fakeMgmtBrLinkMgmtBrInstance = &fakeMgmtBridgeLinkNode{
			GenericLinkNode: GenericLinkNode{
				shortname: "mgmt-net",
				endpoints: []Endpoint{},
				nspath:    nspath,
			},
		}
	}
	return _fakeMgmtBrLinkMgmtBrInstance
}

func GetFakeMgmtBrLinkNode() Node { // skipcq: RVV-B0001
	return getFakeMgmtBrLinkNode()
}

func SetMgmtNetUnderlayingBridge(bridge string) error {
	getFakeMgmtBrLinkNode().GenericLinkNode.shortname = bridge
	return nil
}

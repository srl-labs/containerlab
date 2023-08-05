package types

import (
	"fmt"
)

type LinkMgmtNetRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (r *LinkMgmtNetRaw) ToLinkConfig() *LinkBrief {
	lc := &LinkBrief{
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

func (r *LinkMgmtNetRaw) Resolve(params *ResolveParams) (LinkInterf, error) {

	// create the LinkMgmtNet struct
	link := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
	}

	fakeMgmtBridgeNode := GetFakeMgmtBrLinkNode(params.MgmtBridgeName)

	bridgeEp := &EndptBridge{
		EndptGeneric: EndptGeneric{
			Node:  fakeMgmtBridgeNode,
			state: EndptDeployStateDeployed,
			Iface: r.HostInterface,
			Link:  link,
		},
		masterInterface: params.MgmtBridgeName,
	}

	// add endpoint to fake mgmt bridge node
	err := fakeMgmtBridgeNode.AddEndpoint(bridgeEp)
	if err != nil {
		return nil, err
	}

	// resolve and populate the endpoint
	contEp, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}

	link.Endpoints = []Endpt{bridgeEp, contEp}

	return link, nil
}

func (r *LinkMgmtNetRaw) GetType() LinkType {
	return LinkTypeMgmtNet
}

func mgmtNetFromLinkConfig(lc LinkBrief, specialEPIndex int) (*LinkMgmtNetRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &LinkMgmtNetRaw{
		LinkCommonParams: LinkCommonParams{
			MTU:    lc.MTU,
			Labels: lc.Labels,
			Vars:   lc.Vars,
		},
		HostInterface: hostIf,
		Endpoint:      NewEndpointRaw(node, nodeIf, ""),
	}
	return result, nil
}

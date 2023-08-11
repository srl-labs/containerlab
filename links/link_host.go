package links

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

// LinkHostRaw is the raw (string) representation of a host link as defined in the topology file.
type LinkHostRaw struct {
	LinkCommonParams `yaml:",inline"`
	HostInterface    string       `yaml:"host-interface"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

// ToLinkBrief converts the raw link into a LinkConfig.
func (r *LinkHostRaw) ToLinkBrief() *LinkBriefRaw {
	lc := &LinkBriefRaw{
		Endpoints: make([]string, 2),
		LinkCommonParams: LinkCommonParams{
			MTU:    r.MTU,
			Labels: r.Labels,
			Vars:   r.Vars,
		},
	}

	lc.Endpoints[0] = fmt.Sprintf("%s:%s", r.Endpoint.Node, r.Endpoint.Iface)
	lc.Endpoints[1] = fmt.Sprintf("%s:%s", "host", r.HostInterface)

	return lc
}

func hostLinkFromBrief(lb *LinkBriefRaw, specialEPIndex int) (*LinkHostRaw, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lb, specialEPIndex)

	result := &LinkHostRaw{
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

func (r *LinkHostRaw) GetType() LinkType {
	return LinkTypeHost
}

func (r *LinkHostRaw) Resolve(params *ResolveParams) (Link, error) {
	link := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
	}
	// resolve and populate the endpoint
	ep, err := r.Endpoint.Resolve(params, link)
	if err != nil {
		return nil, err
	}
	hostEp := &EndpointHost{
		EndpointGeneric: *NewEndpointGeneric(GetHostLinkNode(), r.HostInterface),
	}
	hostEp.Link = link

	hostEp.MAC, err = utils.GenMac(ClabOUI)
	if err != nil {
		return nil, err
	}
	// set the end point in the link
	link.Endpoints = []Endpoint{ep, hostEp}

	return link, nil
}

var _hostLinkNodeInstance *hostLinkNode

// hostLinkNode represents a host node which is implicitly used when
// a host link is defined in the topology file.
type hostLinkNode struct {
	GenericLinkNode
}

func (*hostLinkNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeHost
}

// GetHostLinkNode returns the host link node singleton.
func GetHostLinkNode() Node {
	if _hostLinkNodeInstance == nil {
		currns, err := ns.GetCurrentNS()
		if err != nil {
			log.Error(err)
		}
		nspath := currns.Path()

		_hostLinkNodeInstance = &hostLinkNode{
			GenericLinkNode: GenericLinkNode{shortname: "host",
				endpoints: []Endpoint{},
				nspath:    nspath,
			},
		}
	}
	return _hostLinkNodeInstance
}

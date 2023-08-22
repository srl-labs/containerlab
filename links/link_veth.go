package links

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// LinkVEthRaw is the raw (string) representation of a veth link as defined in the topology file.
type LinkVEthRaw struct {
	LinkCommonParams `yaml:",inline"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

// ToLinkBriefRaw converts the raw link into a LinkConfig.
func (r *LinkVEthRaw) ToLinkBriefRaw() *LinkBriefRaw {
	lc := &LinkBriefRaw{
		Endpoints: []string{},
		LinkCommonParams: LinkCommonParams{
			MTU:    r.MTU,
			Labels: r.Labels,
			Vars:   r.Vars,
		},
	}

	for _, e := range r.Endpoints {
		lc.Endpoints = append(lc.Endpoints, fmt.Sprintf("%s:%s", e.Node, e.Iface))
	}
	return lc
}

func (*LinkVEthRaw) GetType() LinkType {
	return LinkTypeVEth
}

// Resolve resolves the raw veth link definition into a Link interface that is implemented
// by a concrete LinkVEth struct.
// Resolving a veth link resolves its endpoints.
func (r *LinkVEthRaw) Resolve(params *ResolveParams) (Link, error) {
	// filtered true means the link is in the filter provided by a user
	// aka it should be resolved/created/deployed
	filtered := isInFilter(params, r.Endpoints)
	if !filtered {
		return nil, nil
	}

	// create LinkVEth struct
	l := &LinkVEth{
		LinkCommonParams: r.LinkCommonParams,
		Endpoints:        make([]Endpoint, 0, 2),
	}

	// resolve raw endpoints (epr) to endpoints (ep)
	for _, epr := range r.Endpoints {
		ep, err := epr.Resolve(params, l)
		if err != nil {
			return nil, err
		}
		// add endpoint to the link endpoints
		l.Endpoints = append(l.Endpoints, ep)
		// add link to endpoint node
		ep.GetNode().AddLink(l)
	}

	return l, nil
}

// linkVEthRawFromLinkBriefRaw creates a raw veth link from a LinkBriefRaw.
func linkVEthRawFromLinkBriefRaw(lb *LinkBriefRaw) (*LinkVEthRaw, error) {
	host, hostIf, node, nodeIf := extractHostNodeInterfaceData(lb, 0)

	result := &LinkVEthRaw{
		LinkCommonParams: LinkCommonParams{
			MTU:    lb.MTU,
			Labels: lb.Labels,
			Vars:   lb.Vars,
		},
		Endpoints: []*EndpointRaw{
			NewEndpointRaw(host, hostIf, ""),
			NewEndpointRaw(node, nodeIf, ""),
		},
	}
	return result, nil
}

type LinkVEth struct {
	LinkCommonParams
	Endpoints []Endpoint

	deploymentState LinkDeploymentState
	deployMutex     sync.Mutex
}

func (*LinkVEth) GetType() LinkType {
	return LinkTypeVEth
}

func (*LinkVEth) Verify() {
}

func (l *LinkVEth) Deploy(ctx context.Context) error {
	// since each node calls deploy on its links, we need to make sure that we only deploy
	// the link once, even if multiple nodes call deploy on the same link.
	l.deployMutex.Lock()
	defer l.deployMutex.Unlock()
	if l.deploymentState == LinkDeploymentStateDeployed {
		return nil
	}

	for _, ep := range l.GetEndpoints() {
		if ep.GetNode().GetState() != state.Deployed {
			return nil
		}
	}

	log.Infof("Creating link: %s <--> %s", l.GetEndpoints()[0], l.GetEndpoints()[1])

	// build the netlink.Veth struct for the link provisioning
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: l.Endpoints[0].GetRandIfaceName(),
			MTU:  l.MTU,
			// Mac address is set later on
		},
		PeerName: l.Endpoints[1].GetRandIfaceName(),
		// PeerMac address is set later on
	}

	// add the link
	err := netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// retrieve the netlink.Link for the B / Peer side of the link
	linkB, err := netlink.LinkByName(l.Endpoints[1].GetRandIfaceName())
	if err != nil {
		return err
	}

	// once veth pair is created, disable tx offload for the veth pair
	for _, ep := range l.Endpoints {
		if err := utils.EthtoolTXOff(ep.GetRandIfaceName()); err != nil {
			return err
		}
	}

	// both ends of the link need to be moved to the relevant network namespace
	// and enabled (up). This is done via linkSetupFunc.
	// based on the endpoint type the link setup function is different.
	// linkSetupFunc is executed in a netns of a node.
	for idx, link := range []netlink.Link{linkA, linkB} {
		// if the node is a regular namespace node
		// add link to node, rename, set mac and Up
		err = l.Endpoints[idx].GetNode().AddLinkToContainer(ctx, link,
			SetNameMACAndUpInterface(link, l.Endpoints[idx]))
		if err != nil {
			return err
		}
	}

	l.deploymentState = LinkDeploymentStateDeployed

	return nil
}

func (*LinkVEth) Remove(_ context.Context) error {
	// TODO
	return nil
}

func (l *LinkVEth) GetEndpoints() []Endpoint {
	return l.Endpoints
}

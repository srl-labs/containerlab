package links

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// LinkVEthRaw is the raw (string) representation of a veth link as defined in the topology file.
type LinkVEthRaw struct {
	LinkCommonParams `yaml:",inline"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

// ToLinkBriefRaw converts the raw link into a LinkBriefRaw.
func (r *LinkVEthRaw) ToLinkBriefRaw() *LinkBriefRaw {
	lc := &LinkBriefRaw{
		Endpoints:        []string{},
		LinkCommonParams: r.LinkCommonParams,
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
	l := NewLinkVEth()
	l.LinkCommonParams = r.LinkCommonParams

	// resolve raw endpoints (epr) to endpoints (ep)
	for _, epr := range r.Endpoints {
		ep, err := epr.Resolve(params, l)
		if err != nil {
			return nil, err
		}
		// add endpoint to the link endpoints
		l.Endpoints = append(l.Endpoints, ep)
	}

	// set default link mtu if MTU is unset
	if l.MTU == 0 {
		l.MTU = DefaultLinkMTU
	}

	return l, nil
}

// linkVEthRawFromLinkBriefRaw creates a raw veth link from a LinkBriefRaw.
func linkVEthRawFromLinkBriefRaw(lb *LinkBriefRaw) (*LinkVEthRaw, error) {
	host, hostIf, node, nodeIf, err := extractHostNodeInterfaceData(lb, 0)
	if err != nil {
		return nil, err
	}

	link := &LinkVEthRaw{
		LinkCommonParams: lb.LinkCommonParams,
		Endpoints: []*EndpointRaw{
			NewEndpointRaw(host, hostIf, ""),
			NewEndpointRaw(node, nodeIf, ""),
		},
	}

	// set default link mtu if MTU is unset
	if link.MTU == 0 {
		link.MTU = DefaultLinkMTU
	}

	return link, nil
}

type LinkVEth struct {
	LinkCommonParams
	Endpoints []Endpoint

	deployMutex sync.Mutex
}

func NewLinkVEth() *LinkVEth {
	return &LinkVEth{
		Endpoints: make([]Endpoint, 0, 2),
	}
}

func (*LinkVEth) GetType() LinkType {
	return LinkTypeVEth
}

func (l *LinkVEth) deployAEnd(ctx context.Context, ep Endpoint) error {

	// Get the index of the Endpoint in the links endpoint slice
	idx, err := l.getEndpointSliceIndex(ep)
	if err != nil {
		return err
	}

	// the peer Endpoint is the other of the two endpoints in the
	// Endpoints slice. So do a +1 on the index and modulo operation
	// to take care of the wrap around.
	peerEp := l.Endpoints[(idx+1)%2]

	log.Infof("Creating Endpoint: %s ( --> %s )", ep, peerEp)

	// build the netlink.Veth struct for the link provisioning
	linkA := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: ep.GetRandIfaceName(),
			MTU:  l.MTU,
			// Mac address is set later on
		},
		PeerName: peerEp.GetRandIfaceName(),
		// PeerMac address is set later on
	}

	// add the link
	err = netlink.LinkAdd(linkA)
	if err != nil {
		return err
	}

	// disable TXOffloading
	if err := utils.EthtoolTXOff(ep.GetRandIfaceName()); err != nil {
		return err
	}

	// the link needs to be moved to the relevant network namespace
	// and enabled (up). This is done via linkSetupFunc.
	// based on the endpoint type the link setup function is different.
	// linkSetupFunc is executed in a netns of a node.
	// if the node is a regular namespace node
	// add link to node, rename, set mac and Up
	err = ep.GetNode().AddLinkToContainer(ctx, linkA,
		SetNameMACAndUpInterface(linkA, ep))
	if err != nil {
		return err
	}

	return nil
}

func (l *LinkVEth) deployBEnd(ctx context.Context, ep Endpoint) error {

	idx, err := l.getEndpointSliceIndex(ep)
	if err != nil {
		return err
	}

	peerEp := l.Endpoints[(idx+1)%2]

	log.Infof("Assigning Endpoint: %s ( --> %s )", ep, peerEp)

	// retrieve the netlink.Link for the provided Endpoint
	link, err := utils.LinkByNameOrAlias(ep.GetRandIfaceName())
	if err != nil {
		return err
	}

	// disable TXOffloading
	if err := utils.EthtoolTXOff(ep.GetRandIfaceName()); err != nil {
		return err
	}

	// the link needs to be moved to the relevant network namespace
	// and enabled (up). This is done via linkSetupFunc.
	// based on the endpoint type the link setup function is different.
	// linkSetupFunc is executed in a netns of a node.
	// if the node is a regular namespace node
	// add link to node, rename, set mac and Up
	err = ep.GetNode().AddLinkToContainer(ctx, link,
		SetNameMACAndUpInterface(link, ep))
	if err != nil {
		return err
	}

	return nil
}

// getEndpointSliceIndex returns the index of the LinkVEth internal ENdpoint slie of the given Endpoint.
// An error is returned when the Endpoint is not part of the Links assigned Endpoints.
func (l *LinkVEth) getEndpointSliceIndex(ep Endpoint) (int, error) {
	// init epIndex with -1 (== not found)
	epIndex := -1
	for idx, e := range l.Endpoints {
		if e == ep {
			epIndex = idx
			break
		}
	}
	// if the ep is not found, return -1 and an error
	if epIndex == -1 {
		// build a string list of endpoints for usefull error output
		epStrings := []string{}
		for _, e := range l.Endpoints {
			epStrings = append(epStrings, e.String())
		}
		// return the error
		return epIndex, fmt.Errorf("endpoint %s does not belong to link [ %s ]", ep.String(), strings.Join(epStrings, ", "))
	}

	return epIndex, nil
}

// Deploy the link based on one of its Endpoints
func (l *LinkVEth) Deploy(ctx context.Context, ep Endpoint) error {
	// since each node calls deploy on its links, we need to make sure that we only deploy
	// the link once, even if multiple nodes call deploy on the same link.
	l.deployMutex.Lock()
	defer l.deployMutex.Unlock()

	// first we need to check that the provided ep is part of this link
	_, err := l.getEndpointSliceIndex(ep)
	if err != nil {
		return err
	}

	// The first node to trigger the link creation will call deployAEnd,
	// subsequent (the second) call will end up in deployBEnd.
	switch l.DeploymentState {
	case LinkDeploymentStateDeployed:
		return l.deployBEnd(ctx, ep)
	default:
		err := l.deployAEnd(ctx, ep)
		l.DeploymentState = LinkDeploymentStateDeployed
		return err
	}
}

func (l *LinkVEth) Remove(_ context.Context) error {
	l.deployMutex.Lock()
	defer l.deployMutex.Unlock()
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	for _, ep := range l.GetEndpoints() {
		err := ep.Remove()
		if err != nil {
			log.Debug(err)
		}
	}
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

func (l *LinkVEth) GetEndpoints() []Endpoint {
	return l.Endpoints
}

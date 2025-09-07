package links

import (
	"context"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/vishvananda/netlink"
)

// LinkDummyRaw is the raw (string) representation of a dummy link as defined in the topology file.
type LinkDummyRaw struct {
	LinkCommonParams `yaml:",inline"`
	Endpoint         *EndpointRaw `yaml:"endpoint"`
}

func (*LinkDummyRaw) GetType() LinkType {
	return LinkTypeVEth
}

// Resolve resolves the raw veth link definition into a Link interface that is implemented
// by a concrete LinkDummyRaw struct.
// Resolving a dummy links endpoints.
func (r *LinkDummyRaw) Resolve(params *ResolveParams) (Link, error) {
	// filtered true means the link is in the filter provided by a user
	// aka it should be resolved/created/deployed
	filtered := isInFilter(params, []*EndpointRaw{r.Endpoint})
	if !filtered {
		return nil, nil
	}

	// create LinkDummyRaw struct
	l := NewLinkDummy()
	l.LinkCommonParams = r.LinkCommonParams

	// resolve raw endpoints (epr) to endpoints (ep)
	ep, err := r.Endpoint.Resolve(params, l)
	if err != nil {
		return nil, err
	}
	// add endpoint to the link endpoints
	l.Endpoints = append(l.Endpoints, ep)

	// set default link mtu if MTU is unset
	if l.MTU == 0 {
		l.MTU = clabconstants.DefaultLinkMTU
	}

	return l, nil
}

type LinkDummy struct {
	LinkCommonParams
	Endpoints []Endpoint
}

func NewLinkDummy() *LinkDummy {
	return &LinkDummy{}
}

func (*LinkDummy) GetType() LinkType {
	return LinkTypeDummy
}

// Deploy deploys the dummy link.
func (l *LinkDummy) Deploy(ctx context.Context, ep Endpoint) error {
	log.Debugf("Creating Endpoint: %s ( --> dummy )", ep)

	// build the netlink.Dummy struct for the link provisioning
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: ep.GetRandIfaceName(),
			MTU:  l.MTU,
			// Mac address is set later on
		},
	}

	// add the link
	err := netlink.LinkAdd(link)
	if err != nil {
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

	l.DeploymentState = LinkDeploymentStateHalfDeployed

	return nil
}

func (l *LinkDummy) Remove(ctx context.Context) error {
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	for _, ep := range l.GetEndpoints() {
		err := ep.Remove(ctx)
		if err != nil {
			log.Debug(err)
		}
	}
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

func (l *LinkDummy) GetEndpoints() []Endpoint {
	return l.Endpoints
}

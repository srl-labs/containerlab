package links

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

type Bridge struct {
	LinkCommonParams
	endpoint Endpoint
}

func NewBridge() *Bridge {
	return &Bridge{}
}

func (b *Bridge) SetEndpoint(e Endpoint) {
	b.endpoint = e
}

// Deploy deploys the link. Endpoint is the endpoint that triggers the creation of the link.
func (b *Bridge) Deploy(ctx context.Context, endpoint Endpoint) error {
	err := endpoint.GetNode().ExecFunction(ctx, func(nn ns.NetNS) error {
		// add the bridge
		err := netlink.LinkAdd(&netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: b.endpoint.GetIfaceName(),
			},
		})
		if err != nil {
			return err
		}
		// retrieve link ref
		netlinkLink, err := netlink.LinkByName(b.endpoint.GetIfaceName())
		if err != nil {
			return err
		}
		// bring the link up
		err = netlink.LinkSetUp(netlinkLink)
		if err != nil {
			return err
		}
		return nil
	})

	return err
}

// Remove removes the link.
func (b *Bridge) Remove(ctx context.Context) error {
	// check Deployment state, if the Link was already
	// removed via e.g. the peer node
	if b.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}
	// trigger link removal via the NodeEndpoint
	err := b.endpoint.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// adjust the Deployment status to reflect the removal
	b.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

// GetType returns the type of the link.
func (b *Bridge) GetType() LinkType {
	return LinkTypeBridge
}

// GetEndpoints returns the endpoints of the link.
func (b *Bridge) GetEndpoints() []Endpoint {
	return []Endpoint{b.endpoint}
}

// GetMTU returns the Link MTU.
func (b *Bridge) GetMTU() int {
	return b.MTU
}

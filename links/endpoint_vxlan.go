package links

import (
	"context"
	"fmt"
	"net"
)

type EndpointVxlan struct {
	EndpointGeneric
	dstPort     int
	srcPort     int
	remote      net.IP
	parentIface string
	vni         int
	randName    string
}

func NewEndpointVxlan(node Node, link Link) *EndpointVxlan {
	return &EndpointVxlan{
		randName: genRandomIfName(),
		EndpointGeneric: EndpointGeneric{
			Link: link,
			Node: node,
		},
	}
}

func (e *EndpointVxlan) String() string {
	return fmt.Sprintf("vxlan remote: %q, dst-port: %d, vni: %d", e.remote, e.dstPort, e.vni)
}

// Verify verifies that the endpoint is valid and can be deployed.
func (e *EndpointVxlan) Verify(_ context.Context, _ *VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}

func (e *EndpointVxlan) Deploy(ctx context.Context) error {
	return e.GetLink().Deploy(ctx, e)
}

func (e *EndpointVxlan) IsNodeless() bool {
	return false
}

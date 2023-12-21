package links

import (
	"fmt"
	"net"
)

type EndpointVxlan struct {
	EndpointGeneric
	udpPort     int
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
	return fmt.Sprintf("vxlan remote: %q, udp-port: %d, vni: %d", e.remote, e.udpPort, e.vni)
}

// Verify verifies that the endpoint is valid and can be deployed.
func (e *EndpointVxlan) Verify(*VerifyLinkParams) error {
	return CheckEndpointUniqueness(e)
}

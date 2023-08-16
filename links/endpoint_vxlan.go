package links

import (
	"fmt"
	"net"
)

type EndpointVxlan struct {
	ifaceName   string
	mac         net.HardwareAddr
	udpPort     int
	remote      net.IP
	parentIface string
	vni         int
	randName    string
	link        Link
}

func NewEndpointVxlan(node Node, link Link) *EndpointVxlan {
	return &EndpointVxlan{
		link:     link,
		randName: genRandomIfName(),
	}
}

func (e *EndpointVxlan) GetNode() Node {
	return nil
}

func (e *EndpointVxlan) GetIfaceName() string {
	return e.ifaceName
}

func (e *EndpointVxlan) GetRandIfaceName() string {
	return e.randName
}

func (e *EndpointVxlan) GetMac() net.HardwareAddr {
	return e.mac
}

func (e *EndpointVxlan) GetLink() Link {
	return e.link
}

func (e *EndpointVxlan) String() string {
	return fmt.Sprintf("vxlan remote: %q, udp-port: %d, vni: %d", e.remote, e.udpPort, e.vni)
}

// // GetLink retrieves the link that the endpoint is assigned to
// func (e *EndpointVxlan) GetLink() Link
// // Verify verifies that the endpoint is valid and can be deployed
func (e *EndpointVxlan) Verify(*VerifyLinkParams) error {
	return nil
}

// // HasSameNodeAndInterface returns true if an endpoint that implements this interface
// // has the same node and interface name as the given endpoint.
func (e *EndpointVxlan) HasSameNodeAndInterface(ept Endpoint) bool {
	return false
}
func (e *EndpointVxlan) Remove() error {
	return nil
}

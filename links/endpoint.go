package links

import (
	"net"

	"github.com/srl-labs/containerlab/nodes"
)

type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	Mac   string `yaml:"mac"`
}

func (e *EndpointRaw) UnRaw(res Resolver) (*Endpoint, error) {
	n, err := res.ResolveNode(e.Node)
	if err != nil {
		return nil, err
	}

	return NewEndpoint(n, e.Iface, net.HardwareAddr{}), nil // TODO: MAC
}

type Endpoint struct {
	Node       nodes.Node
	Iface      string
	MacAddress net.HardwareAddr
	randName   string
}

func NewEndpoint(n nodes.Node, Iface string, MacAddress net.HardwareAddr) *Endpoint {
	return &Endpoint{
		Node:       n,
		Iface:      Iface,
		MacAddress: MacAddress,
		randName:   "",
	}
}

func (e *Endpoint) GetRandName() string {
	// generate random interface name on the fly if not already generated
	if e.randName == "" {
		e.randName = genRandomIfName()
	}
	return e.randName
}

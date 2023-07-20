package types

import "net"

type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	Mac   string `yaml:"mac,omitempty"`
}

func NewEndpointRaw(node, nodeIf, Mac string) *EndpointRaw {
	return &EndpointRaw{
		Node:  node,
		Iface: nodeIf,
		Mac:   Mac,
	}
}

type Endpt struct {
	Iface string
	Mac   net.HardwareAddr
}

func (e *EndpointRaw) ToEndpt() (*Endpt, error) {
	// TODO: need implementation
	return nil, nil
}

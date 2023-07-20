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

func (e *EndpointRaw) ToEndpt() (*Endpt, error) {
	// TODO: need implementation
	return nil, nil
}

type Endpt struct {
	Node     LinkNode
	Iface    string
	Mac      net.HardwareAddr
	randName string
}

func (e *Endpt) GetRandName() string {
	// generate random interface name on the fly if not already generated
	if e.randName == "" {
		e.randName = genRandomIfName()
	}
	return e.randName
}

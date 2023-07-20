package types

import (
	"fmt"
	"net"
)

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

func (e *EndpointRaw) Resolve(nodes map[string]LinkNode) (*Endpt, error) {
	// check if the referenced node does exist
	node, exists := nodes[e.Node]
	if !exists {
		return nil, fmt.Errorf("unable to find node %s", e.Node)
	}

	// create the result struct
	result := &Endpt{
		Node:  node,
		Iface: e.Iface,
	}

	// if MAC is present, set it
	if e.Mac != "" {
		m, err := net.ParseMAC(e.Mac)
		if err != nil {
			return nil, err
		}
		result.Mac = m
	}

	return result, nil
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

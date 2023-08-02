package types

import (
	"context"
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

func (e *EndpointRaw) Resolve(nodes map[string]LinkNode, l LinkInterf) (*EndptGeneric, error) {
	// check if the referenced node does exist
	node, exists := nodes[e.Node]
	if !exists {
		return nil, fmt.Errorf("unable to find node %s", e.Node)
	}

	// create the result struct
	result := &EndptGeneric{
		Node:  node,
		Iface: e.Iface,
		Link:  l,
	}

	// also add the endpoint to the node
	err := node.AddEndpoint(result)
	if err != nil {
		return nil, err
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

type EndptGeneric struct {
	Node     LinkNode
	Iface    string
	Link     LinkInterf
	Mac      net.HardwareAddr
	randName string
	state    EndptDeployState
}

func (e *EndptGeneric) GetRandIfaceName() string {
	// generate random interface name on the fly if not already generated
	if e.randName == "" {
		e.randName = genRandomIfName()
	}
	return e.randName
}

func (e *EndptGeneric) GetIfaceName() string {
	return e.Iface
}

func (e *EndptGeneric) GetMac() net.HardwareAddr {
	return e.Mac
}

func (e *EndptGeneric) GetLink() LinkInterf {
	return e.Link
}

func (e *EndptGeneric) GetNode() LinkNode {
	return e.Node
}

func (e *EndptGeneric) Deploy(ctx context.Context) error {
	e.state = EndptDeployStateReady
	return e.Link.Deploy(ctx)
}

func (e *EndptGeneric) String() string {
	return fmt.Sprintf("Endpoint: %s:%s", e.Node.GetShortName(), e.Iface)
}

type EndptDeployState int8

const (
	EndptDeployStateNotReady = iota
	EndptDeployStateReady
	EndptDeployStateDeployed
)

type Endpt interface {
	GetNode() LinkNode
	GetIfaceName() string
	GetRandIfaceName() string
	GetMac() net.HardwareAddr
	Deploy(ctx context.Context) error
	String() string
	GetLink() LinkInterf
}

// type EndptBridge struct {
// 	EndptGeneric
// }

// func (*EndptBridge) Deploy(ctx context.Context) error {
// 	// NOOP
// 	return nil
// }

// type EndptHost struct {
// 	EndptGeneric
// }

// func (*EndptHost) Deploy(ctx context.Context) error {
// 	// NOOP
// 	return nil
// }

// type EndptMacVlan struct {
// 	EndptGeneric
// }

// func (*EndptMacVlan) Deploy(ctx context.Context) error {
// 	// NOOP
// 	return nil
// }

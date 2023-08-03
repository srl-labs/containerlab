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

func (e *EndpointRaw) Resolve(nodes map[string]LinkNode, l LinkInterf) (Endpt, error) {
	// check if the referenced node does exist
	node, exists := nodes[e.Node]
	if !exists {
		return nil, fmt.Errorf("unable to find node %s", e.Node)
	}

	// create the result struct
	genericEndpt := &EndptGeneric{
		Node:  node,
		Iface: e.Iface,
		Link:  l,
	}

	// if MAC is present, set it
	if e.Mac != "" {
		m, err := net.ParseMAC(e.Mac)
		if err != nil {
			return nil, err
		}
		genericEndpt.Mac = m
	}

	var finalEndpt Endpt = genericEndpt

	switch node.GetLinkEndpointType() {
	case LinkEndpointTypeBridge:
		finalEndpt = &EndptBridge{
			EndptGeneric: *genericEndpt,
		}
	case LinkEndpointTypeHost:
		finalEndpt = &EndptHost{
			EndptGeneric: *genericEndpt,
		}
	case LinkEndpointTypeRegular:
		// NOOP - use EndpointGeneric
	}

	// also add the endpoint to the node
	err := node.AddEndpoint(finalEndpt)
	if err != nil {
		return nil, err
	}

	return finalEndpt, nil
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

func (e *EndptGeneric) GetState() EndptDeployState {
	return e.state
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

func (e *EndptGeneric) Verify(epts []Endpt) error {
	for _, ept := range epts {
		if e.IsSameNodeInterface(ept) {
			return fmt.Errorf("duplicate endpoint %s:%s", e.GetNode().GetShortName(), e.Iface)
		}
	}
	return nil
}

func (e *EndptGeneric) IsSameNodeInterface(ept Endpt) bool {
	return e.Node == ept.GetNode() && e.Iface == ept.GetIfaceName()
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
	Verify([]Endpt) error
	IsSameNodeInterface(ept Endpt) bool
	GetState() EndptDeployState
}

type EndptBridge struct {
	EndptGeneric
}

func (e *EndptBridge) Verify(epts []Endpt) error {
	// TODO:
	// check bridge exists
	return nil
}

type EndptHost struct {
	EndptGeneric
}

func (e *EndptHost) Verify(epts []Endpt) error {
	// TODO:
	// check
	return nil
}

type EndptMacVlan struct {
	EndptGeneric
}

type EndptVeth struct {
	EndptGeneric
}

func (e *EndptVeth) Verify(epts []Endpt) error {
	for _, ept := range epts {
		if e == ept {
			// epts contains all endpoints, hence also the
			// one we're checking here. So if ept is pointer equal to e,
			// we continue with next ept
			continue
		}

		if e.IsSameNodeInterface(ept) {
			return fmt.Errorf("duplicate endpoint %s:%s", e.GetNode().GetShortName(), e.Iface)
		}
	}
	return nil
}

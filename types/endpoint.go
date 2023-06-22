package types

import (
	"net"

	"github.com/srl-labs/containerlab/utils"
)

type LinkNode interface {
	GetNamespacePath() string
	GetNodeName() string
	//GetType() EndpointType
}

// type EndpointType string

// const (
// 	EndpointTypeVarious   EndpointType = "various"
// 	EndpointTypeBridge    EndpointType = "bridge"
// 	EndpointTypeHost      EndpointType = "host"
// 	EndpointTypeOvsBridge EndpointType = "ovs-bridge"
// )

type EndpointRaw struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
	Mac   string `yaml:"mac"`
}

func (e *EndpointRaw) Resolve(res NodeResolver) (*Endpoint, error) {
	n, err := res.ResolveNode(e.Node)
	if err != nil {
		return nil, err
	}

	var mac net.HardwareAddr = nil
	if len(e.Mac) > 0 {
		mac, err = net.ParseMAC(e.Mac)
		if err != nil {
			return nil, err
		}
	}
	return NewEndpoint(n, e.Iface, mac), nil
}

type Endpoint struct {
	Node       LinkNode
	Iface      string
	MacAddress net.HardwareAddr
	randName   string
}

func NewEndpoint(n LinkNode, Iface string, MacAddress net.HardwareAddr) *Endpoint {
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

func (e *Endpoint) DisableTxOffload(linkName TxOffloadLinkName) error {
	intfName := ""
	switch linkName {
	case TxOffloadLinkNameFinal:
		intfName = e.Iface
	case TxOffloadLinkNameRandom:
		intfName = e.GetRandName()
	}
	return utils.EthtoolTXOff(intfName)
}

type TxOffloadLinkName int

const (
	TxOffloadLinkNameRandom = iota
	TxOffloadLinkNameFinal  = iota
)

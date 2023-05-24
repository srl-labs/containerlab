package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"gopkg.in/yaml.v2"
)

type Link interface {
	//Deploy() error
	GetType() (LinkType, error)
}

type MacVLanLink struct {
	macVXType `yaml:",inline"`
}

type RawLinkTypeAlias RawLinkType

type MacVTapLink struct {
	macVXType `yaml:",inline"`
}

type HostLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
}

type macVXType struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
	MAC              string `yaml:"mac"`
	LinkStatus
}

func (m *macVXType) Deploy(iftype LinkType) error {
	parentInterface, err := netlink.LinkByName(m.HostInterface)
	if err != nil {
		return err
	}

	mvl := netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        m.GetRandName(),
			ParentIndex: parentInterface.Attrs().Index,
		},
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}

	var link netlink.Link
	switch iftype {
	case LinkTypeMacVTap:
		link = &netlink.Macvtap{Macvlan: mvl}
	case LinkTypeMacVLan:
		link = &mvl
	}

	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	var mvInterface netlink.Link
	if mvInterface, err = netlink.LinkByName(m.GetRandName()); err != nil {
		return fmt.Errorf("failed to lookup %q: %v", m.GetRandName(), err)
	}

	err = netlink.LinkSetHardwareAddr(mvInterface, net.HardwareAddr(m.MAC))
	if err != nil {
		return err
	}

	return nil
}

type VEthLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	Mtu              int         `yaml:"mtu,omitempty"`
	Endpoints        []*Endpoint `yaml:"endpoints"`
}

type Endpoint struct {
	Node  string `yaml:"node"`
	Iface string `yaml:"interface"`
}

type MgmtNetLink struct {
	RawLinkTypeAlias `yaml:",inline"`
	HostInterface    string `yaml:"host-interface"`
	Node             string `yaml:"node"`
	NodeInterface    string `yaml:"node-interface"`
}

type RawLinkType struct {
	Type     string                 `yaml:"type"`
	Labels   map[string]string      `yaml:"labels,omitempty"`
	Vars     map[string]interface{} `yaml:"vars,omitempty"`
	Instance interface{}
}

type LinkStatus struct {
	randName string
	state    LinkState
}

func (l *LinkStatus) GetState() LinkState {
	return l.state
}

func (l *LinkStatus) GetRandName() string {
	if l.randName == "" {
		l.randName = fmt.Sprintf("clab-%s", genIfName())
	}
	return l.randName
}

func genIfName() string {
	s, _ := uuid.New().MarshalText() // .MarshalText() always return a nil error
	return string(s[:8])
}

type LinkState int

const (
	UnDeployed LinkState = iota
	Deployed   LinkState = iota
	Up         LinkState = iota
)

type LinkType string

const (
	LinkTypeVEth    LinkType = "veth"
	LinkTypeMgmtNet LinkType = "mgmt-net"
	LinkTypeMacVLan LinkType = "macvlan"
	LinkTypeMacVTap LinkType = "macvtap"
	LinkTypeHost    LinkType = "host"

	LinkTypeDeprecate LinkType = "DEPRECATE"
)

func ParseLinkType(s string) (LinkType, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case string(LinkTypeMacVLan):
		return LinkTypeMacVLan, nil
	case string(LinkTypeMacVTap):
		return LinkTypeMacVTap, nil
	case string(LinkTypeVEth):
		return LinkTypeVEth, nil
	case string(LinkTypeMgmtNet):
		return LinkTypeMgmtNet, nil
	case string(LinkTypeDeprecate):
		return LinkTypeDeprecate, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

func (rlt *RawLinkTypeAlias) GetType() (LinkType, error) {
	return ParseLinkType(rlt.Type)
}

type ClabConfig struct {
	Nodes map[string]interface{} `yaml:"nodes"`
	Links []*RawLinkType         `yaml:"links"`
}

type Links []*RawLinkType

var _ yaml.Unmarshaler = &RawLinkType{}

func (r *RawLinkType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rtAlias RawLinkTypeAlias

	err := unmarshal(&rtAlias)
	if err != nil {
		return err
	}

	r.Type = rtAlias.Type

	switch strings.ToLower(rtAlias.Type) {
	case "veth":
		var l VEthLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l
	case "mgmt-net":
		var l MgmtNetLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l
	case "host":
		var l HostLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l
	case "macvlan":
		var l MacVLanLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l
	case "macvtap":
		var l MacVTapLink
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Instance = l
	default:
		// try to parse the depricate format
		var l types.LinkConfig
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		r.Type = "DEPRECATE"
		r.Instance, err = deprecateLinkConversion(l)
		if err != nil {
			return err
		}
	}

	return nil
}

func deprecateLinkConversion(lc types.LinkConfig) (Link, error) {
	// check two endpoints defined
	if len(lc.Endpoints) != 2 {
		return nil, fmt.Errorf("endpoint definition should consist of exactly 2 entries. %d provided", len(lc.Endpoints))
	}
	for x, v := range lc.Endpoints {
		parts := strings.SplitN(v, ":", 2)
		node := parts[0]

		lt, err := ParseLinkType(node)
		if err == nil {
			continue
		}
		switch lt {
		case LinkTypeMacVLan:
			return macVlanFromLinkConfig(lc, x)
		case LinkTypeMacVTap:
			return macVTapFromLinkConfig(lc, x)
		case LinkTypeMgmtNet:
			return mgmtNetFromLinkConfig(lc, x)
		case LinkTypeHost:
			return hostFromLinkConfig(lc, x)
		}
	}
	return vEthFromLinkConfig(lc)
}

func vEthFromLinkConfig(lc types.LinkConfig) (*VEthLink, error) {
	nodeA, nodeAIf, nodeB, nodeBIf := extractHostNodeInterfaceData(lc, 0)

	result := &VEthLink{
		RawLinkTypeAlias: RawLinkTypeAlias{
			Type:     string(LinkTypeVEth),
			Labels:   lc.Labels,
			Vars:     lc.Vars,
			Instance: nil,
		},
		Mtu: lc.MTU,
		Endpoints: []*Endpoint{
			{
				Node:  nodeA,
				Iface: nodeAIf,
			},
			{
				Node:  nodeB,
				Iface: nodeBIf,
			},
		},
	}
	return result, nil
}

func mgmtNetFromLinkConfig(lc types.LinkConfig, specialEPIndex int) (*MgmtNetLink, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &MgmtNetLink{
		RawLinkTypeAlias: RawLinkTypeAlias{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
		HostInterface:    hostIf,
		Node:             node,
		NodeInterface:    nodeIf,
	}
	return result, nil
}

func macVXTypeFromLinkConfig(lc types.LinkConfig, specialEPIndex int) (*macVXType, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &macVXType{
		RawLinkTypeAlias: RawLinkTypeAlias{Type: string(LinkTypeMgmtNet), Labels: lc.Labels, Vars: lc.Vars, Instance: nil},
		HostInterface:    hostIf,
		Node:             node,
		NodeInterface:    nodeIf,
	}
	return result, nil
}

func macVlanFromLinkConfig(lc types.LinkConfig, specialEPIndex int) (*MacVLanLink, error) {
	macvx, err := macVXTypeFromLinkConfig(lc, specialEPIndex)
	if err != nil {
		return nil, err
	}

	return &MacVLanLink{macVXType: *macvx}, nil
}

func macVTapFromLinkConfig(lc types.LinkConfig, specialEPIndex int) (*MacVTapLink, error) {
	macvx, err := macVXTypeFromLinkConfig(lc, specialEPIndex)
	if err != nil {
		return nil, err
	}

	return &MacVTapLink{macVXType: *macvx}, nil
}

func extractHostNodeInterfaceData(lc types.LinkConfig, specialEPIndex int) (host string, hostIf string, node string, nodeIf string) {
	// the index of the node is the specialEndpointIndex +1 modulo 2
	nodeindex := (specialEPIndex + 1) % 2

	hostData := strings.SplitN(lc.Endpoints[specialEPIndex], ":", 2)
	nodeData := strings.SplitN(lc.Endpoints[nodeindex], ":", 2)

	host = hostData[0]
	hostIf = hostData[1]
	node = nodeData[0]
	nodeIf = nodeData[1]

	return host, hostIf, node, nodeIf
}

func hostFromLinkConfig(lc types.LinkConfig, specialEPIndex int) (Link, error) {
	_, hostIf, node, nodeIf := extractHostNodeInterfaceData(lc, specialEPIndex)

	result := &HostLink{
		RawLinkTypeAlias: RawLinkTypeAlias{
			Type:     string(LinkTypeHost),
			Labels:   lc.Labels,
			Vars:     lc.Vars,
			Instance: nil,
		},
		HostInterface: hostIf,
		Node:          node,
		NodeInterface: nodeIf,
	}
	return result, nil
}

var yamlData = `
nodes:
    bla: foo
    blubb: peng
links: 
    - endpoints: ["srl:eth1", "srl2:eth3"]
    - type: veth
      mtu: 1500
      endpoints:
      - node:          srl1
        interface:     ethernet-1/1
      - node:        srl2
        interface:    ethernet-1/1
    - type: host
      host-interface:    srl1_e1-2
      node:             srl1
      node-interface:    ethernet-1/2
      labels:
        foo: bar
    - type: macvlan
      host-interface:    eno0
      node:             srl1
      node-interface:    ethernet-1/3
    - type: macvtap
      host-interface:    eno0
      node:             srl1
      node-interface:    ethernet-1/4
    - type: mgmt-net
      host-interface:    srl1_e1-5
      node:             srl1
      node-interface:    ethernet-1/5
`

func main() {
	var c ClabConfig
	err := yaml.Unmarshal([]byte(yamlData), &c)
	if err != nil {
		log.Error(err)
	}
	fmt.Println("Done")
}

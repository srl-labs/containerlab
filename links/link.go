package links

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/srl-labs/containerlab/internal/slices"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/vishvananda/netlink"
	"gopkg.in/yaml.v2"
)

type LinkDeploymentState uint8

const (
	LinkDeploymentStateNotDeployed = iota
	LinkDeploymentStateDeployed
)

// LinkCommonParams represents the common parameters for all link types.
type LinkCommonParams struct {
	MTU    int                    `yaml:"mtu,omitempty"`
	Labels map[string]string      `yaml:"labels,omitempty"`
	Vars   map[string]interface{} `yaml:"vars,omitempty"`
}

// LinkDefinition represents a link definition in the topology file.
type LinkDefinition struct {
	Type string  `yaml:"type,omitempty"`
	Link RawLink `yaml:",inline"`
}

// LinkType represents the type of a link definition.
type LinkType string

const (
	LinkTypeVEth    LinkType = "veth"
	LinkTypeMgmtNet LinkType = "mgmt-net"
	LinkTypeMacVLan LinkType = "macvlan"
	LinkTypeHost    LinkType = "host"

	// LinkTypeBrief is a link definition where link types
	// are encoded in the endpoint definition as string and allow users
	// to quickly type out link endpoints in a yaml file.
	LinkTypeBrief LinkType = "brief"
)

// parseLinkType parses a string representation of a link type into a LinkDefinitionType.
func parseLinkType(s string) (LinkType, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case string(LinkTypeMacVLan):
		return LinkTypeMacVLan, nil
	case string(LinkTypeVEth):
		return LinkTypeVEth, nil
	case string(LinkTypeMgmtNet):
		return LinkTypeMgmtNet, nil
	case string(LinkTypeHost):
		return LinkTypeHost, nil
	case string(LinkTypeBrief):
		return LinkTypeBrief, nil
	default:
		return "", fmt.Errorf("unable to parse %q as LinkType", s)
	}
}

var _ yaml.Unmarshaler = (*LinkDefinition)(nil)

// UnmarshalYAML deserializes links passed via topology file into LinkDefinition struct.
// It supports both the brief and specific link type notations.
func (ld *LinkDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// alias struct to avoid recursion used only need to unmarshal
	// the type field.
	var a struct {
		Type string `yaml:"type"`
	}

	// yaml.TypeError is returned when the yaml parser encounters
	// an unknown field. We want to ignore this error and continue
	// parsing the rest of the fields as we only care about the type field
	// in the a struct.
	var e *yaml.TypeError

	err := unmarshal(&a)
	if err != nil && !errors.As(err, &e) {
		return err
	}

	var lt LinkType

	// if no type is specified, we assume that brief notation of a link definition is used.
	if a.Type == "" {
		lt = LinkTypeBrief
		ld.Type = string(LinkTypeBrief)
	} else {
		ld.Type = a.Type

		lt, err = parseLinkType(a.Type)
		if err != nil {
			return err
		}
	}

	switch lt {
	case LinkTypeVEth:
		var l struct {
			// the Type field is injected artificially
			// to allow strict yaml parsing to work.
			Type        string `yaml:"type"`
			LinkVEthRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		ld.Link = &l.LinkVEthRaw
	case LinkTypeMgmtNet:
		var l struct {
			Type           string `yaml:"type"`
			LinkMgmtNetRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		ld.Link = &l.LinkMgmtNetRaw
	case LinkTypeHost:
		var l struct {
			Type        string `yaml:"type"`
			LinkHostRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		ld.Link = &l.LinkHostRaw
	case LinkTypeMacVLan:
		var l struct {
			Type           string `yaml:"type"`
			LinkMacVlanRaw `yaml:",inline"`
		}
		err := unmarshal(&l)
		if err != nil {
			return err
		}
		ld.Link = &l.LinkMacVlanRaw
	case LinkTypeBrief:
		// brief link's endpoint format
		var l struct {
			Type         string `yaml:"type"`
			LinkBriefRaw `yaml:",inline"`
		}

		err := unmarshal(&l)
		if err != nil {
			return err
		}

		ld.Type = string(LinkTypeBrief)

		ld.Link, err = l.LinkBriefRaw.ToTypeSpecificRawLink()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown link type %q", lt)
	}

	return nil
}

// MarshalYAML serializes LinkDefinition (e.g when used with generate command).
// As of now it falls back to converting the LinkConfig into a
// RawVEthLink, such that the generated LinkConfigs adhere to the new LinkDefinition
// format instead of the brief one.
func (r *LinkDefinition) MarshalYAML() (interface{}, error) {
	switch r.Link.GetType() {
	case LinkTypeHost:
		x := struct {
			LinkHostRaw `yaml:",inline"`
			Type        string `yaml:"type"`
		}{
			LinkHostRaw: *r.Link.(*LinkHostRaw),
			Type:        string(LinkTypeVEth),
		}
		return x, nil
	case LinkTypeVEth:
		x := struct {
			// the Type field is injected artificially
			// to allow strict yaml parsing to work.
			Type        string `yaml:"type"`
			LinkVEthRaw `yaml:",inline"`
		}{
			LinkVEthRaw: *r.Link.(*LinkVEthRaw),
			Type:        string(LinkTypeVEth),
		}
		return x, nil
	case LinkTypeMgmtNet:
		x := struct {
			Type           string `yaml:"type"`
			LinkMgmtNetRaw `yaml:",inline"`
		}{
			LinkMgmtNetRaw: *r.Link.(*LinkMgmtNetRaw),
			Type:           string(LinkTypeMgmtNet),
		}
		return x, nil
	case LinkTypeMacVLan:
		x := struct {
			Type           string `yaml:"type"`
			LinkMacVlanRaw `yaml:",inline"`
		}{
			LinkMacVlanRaw: *r.Link.(*LinkMacVlanRaw),
			Type:           string(LinkTypeMacVLan),
		}
		return x, nil
	case LinkTypeBrief:
		return r.Link, nil
	}

	return nil, fmt.Errorf("unable to marshall")
}

// RawLink is an interface that all raw link types must implement.
// Raw link types define the links as they are defined in the topology file
// and solely a product of unmarshalling.
// Raw links are later "resolved" to concrete link types (e.g LinkVeth).
type RawLink interface {
	Resolve(params *ResolveParams) (Link, error)
	GetType() LinkType
}

// Link is an interface that all concrete link types must implement.
// Concrete link types are resolved from raw links and become part of CLab.Links.
type Link interface {
	// Deploy deploys the link.
	Deploy(context.Context) error
	// Remove removes the link.
	Remove(context.Context) error
	// GetType returns the type of the link.
	GetType() LinkType
	// GetEndpoints returns the endpoints of the link.
	GetEndpoints() []Endpoint
}

func extractHostNodeInterfaceData(lb *LinkBriefRaw, specialEPIndex int) (host, hostIf, node, nodeIf string) {
	// the index of the node is the specialEndpointIndex +1  modulo 2
	nodeindex := (specialEPIndex + 1) % 2

	hostData := strings.SplitN(lb.Endpoints[specialEPIndex], ":", 2)
	nodeData := strings.SplitN(lb.Endpoints[nodeindex], ":", 2)

	host = hostData[0]
	hostIf = hostData[1]
	node = nodeData[0]
	nodeIf = nodeData[1]

	return host, hostIf, node, nodeIf
}

func genRandomIfName() string {
	s, _ := uuid.New().MarshalText() // .MarshalText() always return a nil error
	return "clab-" + string(s[:8])
}

// Node interface is an interface that is satisfied by all nodes.
// It is used a subset of the nodes.Node interface and is used to pass nodes.Nodes
// to the link resolver without causing a circular dependency.
type Node interface {
	// AddLinkToContainer adds a link to the node (container).
	// In case of a regular container, it will push the link into the
	// network namespace and then run the function f within the namespace
	// this is to rename the link, set mtu, set the interface up, e.g. see link.SetNameMACAndUpInterface()
	//
	// In case of a bridge node (ovs or regular linux bridge) it will take the interface and make the bridge
	// the master of the interface and bring the interface up.
	AddLinkToContainer(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error
	AddLink(l Link)
	// AddEndpoint adds the Endpoint to the node
	AddEndpoint(e Endpoint)
	GetLinkEndpointType() LinkEndpointType
	GetShortName() string
	GetEndpoints() []Endpoint
	ExecFunction(func(ns.NetNS) error) error
	GetState() state.NodeState
}

type LinkEndpointType string

const (
	LinkEndpointTypeVeth   = "veth"
	LinkEndpointTypeBridge = "bridge"
	LinkEndpointTypeHost   = "host"
)

// SetNameMACAndUpInterface is a helper function that will bind interface name and Mac
// and return a function that can run in the netns.Do() call for execution in a network namespace.
func SetNameMACAndUpInterface(l netlink.Link, endpt Endpoint) func(ns.NetNS) error {
	return func(_ ns.NetNS) error {
		// rename the given link
		err := netlink.LinkSetName(l, endpt.GetIfaceName())
		if err != nil {
			return fmt.Errorf(
				"failed to rename link: %v", err)
		}

		// lets set the MAC address if provided
		if len(endpt.GetMac()) == 6 {
			err = netlink.LinkSetHardwareAddr(l, endpt.GetMac())
			if err != nil {
				return err
			}
		}

		// bring the given link up
		if err = netlink.LinkSetUp(l); err != nil {
			return fmt.Errorf("failed to set %q up: %v",
				endpt.GetIfaceName(), err)
		}
		return nil
	}
}

// ResolveParams is a struct that is passed to the Resolve() function of a raw link
// to resolve it to a concrete link type.
// Parameters include all nodes of a topology and the name of the management bridge.
type ResolveParams struct {
	Nodes          map[string]Node
	MgmtBridgeName string
	// list of node shortnames that user
	// passed as a node filter
	NodesFilter []string
}

type VerifyLinkParams struct {
	RunBridgeExistsCheck bool
}

func NewVerifyLinkParams() *VerifyLinkParams {
	return &VerifyLinkParams{
		RunBridgeExistsCheck: true,
	}
}

// isInFilter returns true if the endpoints of the link
// are part of the nodes filter which means that the link
// should be resolved and deployed.
// In other words, returning true means that the link should be deployed.
func isInFilter(params *ResolveParams, endpoints []*EndpointRaw) bool {
	// empty filter means that all links should be deployed
	if len(params.NodesFilter) == 0 {
		return true
	}

	for _, e := range endpoints {
		if !slices.Contains(params.NodesFilter, e.Node) {
			return false
		}
	}

	return true
}

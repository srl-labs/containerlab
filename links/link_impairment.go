package links

import "fmt"

const (
	LinkImpairmentModeBridge = "bridge"

	DefaultImpairmentBridgeImage = "ghcr.io/srl-labs/network-multitool:v0.4.1"
)

// LinkImpairment describes an optional helper node inserted between the two
// endpoints of an extended veth link.
type LinkImpairment struct {
	Mode  string `yaml:"mode,omitempty"`
	Name  string `yaml:"name,omitempty"`
	Image string `yaml:"image,omitempty"`
}

// ImpairmentBridgeNode describes a generated node needed by an impaired link.
type ImpairmentBridgeNode struct {
	Name          string
	Image         string
	OriginalNodes []string
}

// ImpairmentExpansion is the result of expanding impaired links into regular
// raw links and generated helper node metadata.
type ImpairmentExpansion struct {
	Links       []*LinkDefinition
	BridgeNodes []*ImpairmentBridgeNode
}

// ImpairmentBridgePredicate decides whether the provided endpoint node names
// require a generated bridge node realization.
type ImpairmentBridgePredicate func(endpointNodes []string) bool

// ExpandImpairments expands veth links that either request impairment bridge
// realization or touch an endpoint that requires bridge-backed links.
func ExpandImpairments(
	linkDefs []*LinkDefinition,
	requiresBridge ImpairmentBridgePredicate,
) (*ImpairmentExpansion, error) {
	result := &ImpairmentExpansion{
		Links: make([]*LinkDefinition, 0, len(linkDefs)),
	}
	seenBridgeNodes := map[string]struct{}{}

	for idx, linkDef := range linkDefs {
		if linkDef == nil || linkDef.Link == nil {
			result.Links = append(result.Links, linkDef)
			continue
		}

		raw, ok := linkDef.Link.(*LinkVEthRaw)
		if !ok {
			result.Links = append(result.Links, linkDef)
			continue
		}

		if len(raw.Endpoints) != 2 && raw.Impairment == nil {
			result.Links = append(result.Links, linkDef)
			continue
		}
		if len(raw.Endpoints) != 2 {
			return nil, fmt.Errorf(
				"impairment bridge links require exactly two endpoints, got %d",
				len(raw.Endpoints),
			)
		}

		endpointNodes := []string{raw.Endpoints[0].Node, raw.Endpoints[1].Node}
		if raw.Impairment == nil && (requiresBridge == nil || !requiresBridge(endpointNodes)) {
			result.Links = append(result.Links, linkDef)
			continue
		}

		bridgeNode, err := buildImpairmentBridgeNode(idx, raw)
		if err != nil {
			return nil, err
		}

		if _, ok := seenBridgeNodes[bridgeNode.Name]; ok {
			return nil, fmt.Errorf("duplicate link impairment bridge node %q", bridgeNode.Name)
		}
		seenBridgeNodes[bridgeNode.Name] = struct{}{}
		result.BridgeNodes = append(result.BridgeNodes, bridgeNode)

		commonParams := raw.LinkCommonParams
		commonParams.IPv4 = nil
		commonParams.IPv6 = nil

		result.Links = append(result.Links,
			newImpairmentBridgeSegment(
				commonParams,
				cloneEndpointRaw(raw.Endpoints[0]),
				NewEndpointRaw(bridgeNode.Name, "eth1", ""),
			),
			newImpairmentBridgeSegment(
				commonParams,
				NewEndpointRaw(bridgeNode.Name, "eth2", ""),
				cloneEndpointRaw(raw.Endpoints[1]),
			),
		)
	}

	return result, nil
}

func buildImpairmentBridgeNode(idx int, raw *LinkVEthRaw) (*ImpairmentBridgeNode, error) {
	mode := ""
	if raw.Impairment != nil {
		mode = raw.Impairment.Mode
	}
	if mode == "" {
		mode = LinkImpairmentModeBridge
	}
	if mode != LinkImpairmentModeBridge {
		return nil, fmt.Errorf("unsupported link impairment mode %q", raw.Impairment.Mode)
	}

	name := ""
	if raw.Impairment != nil {
		name = raw.Impairment.Name
	}
	if name == "" {
		name = fmt.Sprintf("impairment-bridge-%02d", idx+1)
	}

	image := ""
	if raw.Impairment != nil {
		image = raw.Impairment.Image
	}
	if image == "" {
		image = DefaultImpairmentBridgeImage
	}

	return &ImpairmentBridgeNode{
		Name:          name,
		Image:         image,
		OriginalNodes: []string{raw.Endpoints[0].Node, raw.Endpoints[1].Node},
	}, nil
}

func newImpairmentBridgeSegment(
	commonParams LinkCommonParams,
	endpointA *EndpointRaw,
	endpointB *EndpointRaw,
) *LinkDefinition {
	return &LinkDefinition{
		Type: string(LinkTypeVEth),
		Link: &LinkVEthRaw{
			LinkCommonParams: commonParams,
			Endpoints: []*EndpointRaw{
				endpointA,
				endpointB,
			},
		},
	}
}

func cloneEndpointRaw(endpoint *EndpointRaw) *EndpointRaw {
	if endpoint == nil {
		return nil
	}

	var vars map[string]any
	if endpoint.Vars != nil {
		vars = map[string]any{}
		for k, v := range endpoint.Vars {
			vars[k] = v
		}
	}

	return &EndpointRaw{
		Node:  endpoint.Node,
		Iface: endpoint.Iface,
		MAC:   endpoint.MAC,
		IPv4:  endpoint.IPv4,
		IPv6:  endpoint.IPv6,
		Vars:  vars,
	}
}

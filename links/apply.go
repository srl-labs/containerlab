package links

import (
	"sort"
	"strings"
)

// ApplyRuntimeEndpoints returns the runtime-owned endpoints that an apply operation may
// deploy, discover, or remove for a link. Parent and remote-only metadata endpoints are
// intentionally excluded.
func ApplyRuntimeEndpoints(l Link) []Endpoint {
	switch link := l.(type) {
	case *LinkVEth:
		return append([]Endpoint(nil), link.Endpoints...)
	case *LinkMacVlan:
		return []Endpoint{link.NodeEndpoint}
	case *LinkVxlan:
		return []Endpoint{link.localEndpoint}
	case *VxlanStitched:
		endpoints := make([]Endpoint, 0, len(link.vethLink.Endpoints)+1)
		endpoints = append(endpoints, link.vethLink.Endpoints...)
		endpoints = append(endpoints, link.vxlanLink.localEndpoint)
		return endpoints
	case *LinkDummy:
		return append([]Endpoint(nil), link.Endpoints...)
	case *Bridge:
		return []Endpoint{link.endpoint}
	default:
		return materialEndpoints(l.GetEndpoints())
	}
}

func materialEndpoints(endpoints []Endpoint) []Endpoint {
	result := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep == nil || ep.GetIfaceName() == "" {
			continue
		}
		result = append(result, ep)
	}
	return result
}

func endpointTokens(endpoints []Endpoint) string {
	tokens := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		token := endpointToken(ep)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return strings.Join(tokens, ",")
}

func endpointToken(ep Endpoint) string {
	if ep == nil || ep.GetNode() == nil || ep.GetIfaceName() == "" {
		return ""
	}
	nodeName := ep.GetNode().GetShortName()
	if ep.IsNodeless() && ep.GetNode().GetLinkEndpointType() == LinkEndpointTypeBridge {
		nodeName = "mgmt-net"
	}
	return nodeName + ":" + ep.GetIfaceName()
}

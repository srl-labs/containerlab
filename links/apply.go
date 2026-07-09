package links

// ApplyRuntimeEndpoints returns the runtime-owned endpoints that an apply operation may
// deploy, discover, or remove for a link. Parent and remote-only metadata endpoints are
// intentionally excluded.
func ApplyRuntimeEndpoints(l Link) []Endpoint {
	switch link := l.(type) {
	case *LinkVEth:
		return append([]Endpoint(nil), link.Endpoints...)
	case *LinkVEthStitched:
		return []Endpoint{link.segA.Endpoints[0], link.segB.Endpoints[0]}
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

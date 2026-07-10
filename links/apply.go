package links

// ApplyRuntimeEndpoints returns the runtime-owned endpoints that an apply operation may
// deploy, discover, or remove for a link. Parent and remote-only metadata endpoints are
// intentionally excluded.
func ApplyRuntimeEndpoints(l Link) []Endpoint {
	if l == nil {
		return nil
	}

	return materialEndpoints(l.GetRuntimeEndpoints())
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

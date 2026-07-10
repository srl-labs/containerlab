package links

// RuntimeEndpoints returns the material endpoints a deployment may create, discover, or remove.
// Parent and remote-only metadata endpoints are intentionally excluded.
func RuntimeEndpoints(l Link) []Endpoint {
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

package links

import (
	"fmt"
	"strings"
)

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

// RuntimeEndpointsContainEndpoint reports whether link owns a material endpoint
// with the given logical node name and interface name.
func RuntimeEndpointsContainEndpoint(
	l Link,
	nodeName string,
	ifaceName string,
) bool {
	if l == nil {
		return false
	}

	for _, ep := range RuntimeEndpoints(l) {
		node := ep.GetNode()
		if node == nil || node.GetShortName() != nodeName {
			continue
		}
		if ep.GetIfaceName() == ifaceName {
			return true
		}
	}

	return false
}

// RawLinkEndpoints returns (nodeName, ifaceName) pairs for the endpoints of a
// raw link definition. Tolerates the brief string form and the typed raw
// forms; unknown raw types yield no pairs.
func RawLinkEndpoints(rl RawLink) [][2]string {
	switch raw := rl.(type) {
	case *LinkBriefRaw:
		out := make([][2]string, 0, len(raw.Endpoints))
		for _, ep := range raw.Endpoints {
			node, iface, err := SplitEndpointString(ep)
			if err != nil {
				continue
			}
			out = append(out, [2]string{node, iface})
		}
		return out
	case *LinkVEthRaw:
		out := make([][2]string, 0, len(raw.Endpoints))
		for _, ep := range raw.Endpoints {
			out = append(out, [2]string{ep.Node, ep.Iface})
		}
		return out
	default:
		return nil
	}
}

// LinkDeploymentStateSetter is implemented by links whose deployment progress
// can be marked from the outside, e.g. after a migrated bridge port fully
// re-establishes the pair without pair re-creation.
type LinkDeploymentStateSetter interface {
	SetDeploymentState(state LinkDeploymentState)
}

// SplitEndpointString splits "nodeName:ifaceName" into its parts; the iface
// part may itself contain colons (e.g. interface names in brief endpoints).
func SplitEndpointString(endpoint string) (nodeName, ifaceName string, err error) {
	parts := strings.SplitN(endpoint, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid endpoint %q; expected nodeName:iface", endpoint)
	}
	return parts[0], parts[1], nil
}

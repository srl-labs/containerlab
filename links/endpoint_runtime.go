package links

import "context"

// EndpointRuntime represents an interface that exists at runtime but is not backed by the
// original topology link graph.
type EndpointRuntime struct {
	EndpointGeneric
}

func NewRuntimeEndpoint(node Node, ifaceName string) *EndpointRuntime {
	return &EndpointRuntime{
		EndpointGeneric: EndpointGeneric{
			Node:      node,
			IfaceName: ifaceName,
			Vars:      make(map[string]any),
		},
	}
}

func (*EndpointRuntime) Verify(context.Context, *VerifyLinkParams) error {
	return nil
}

func (e *EndpointRuntime) MoveTo(ctx context.Context, dst Node, bringUp bool) error {
	return moveEndpoint(ctx, e, dst, bringUp)
}

func (*EndpointRuntime) IsRuntimeDiscovered() bool {
	return true
}

func (*EndpointRuntime) IsNodeless() bool {
	return false
}

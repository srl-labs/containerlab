package links

import (
	"context"
	"fmt"
)

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

func (e *EndpointRuntime) Deploy(context.Context) error {
	return fmt.Errorf("runtime-discovered endpoint %q has no topology link", e.GetIfaceName())
}

func (*EndpointRuntime) IsRuntimeDiscovered() bool {
	return true
}

func (*EndpointRuntime) IsNodeless() bool {
	return false
}

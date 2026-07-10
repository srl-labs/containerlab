package links

import (
	"context"
	"testing"
)

type deleteTestEndpoint struct {
	EndpointGeneric
	removed bool
}

func (*deleteTestEndpoint) Verify(context.Context, *VerifyLinkParams) error { return nil }

func (*deleteTestEndpoint) HasSameNodeAndInterface(Endpoint) bool { return false }

func (*deleteTestEndpoint) IsNodeless() bool { return false }

func (e *deleteTestEndpoint) Remove(context.Context) error {
	e.removed = true
	return nil
}

func (e *deleteTestEndpoint) MoveTo(ctx context.Context, dst Node) error {
	return moveEndpoint(ctx, e, dst)
}

func TestGenericLinkNodeDeleteHandlesEndpointsWithoutLink(t *testing.T) {
	node := &GenericLinkNode{
		shortname: "test-node",
		endpoints: []Endpoint{},
	}

	ep := &deleteTestEndpoint{
		EndpointGeneric: EndpointGeneric{
			Node:      newFakeNode("test-node"),
			IfaceName: "eth1",
		},
	}

	if err := node.AdoptEndpoint(ep); err != nil {
		t.Fatalf("unexpected adopt error: %v", err)
	}

	if err := node.Delete(context.Background()); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	if !ep.removed {
		t.Fatalf("expected endpoint Remove to be called")
	}
}

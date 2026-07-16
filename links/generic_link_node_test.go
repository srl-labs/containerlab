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

func (*deleteTestEndpoint) Deploy(context.Context) error { return nil }

func (*deleteTestEndpoint) HasSameNodeAndInterface(Endpoint) bool { return false }

func (*deleteTestEndpoint) IsNodeless() bool { return false }

func (e *deleteTestEndpoint) Remove(context.Context) error {
	e.removed = true
	return nil
}

func TestCheckEndpointUniqueness(t *testing.T) {
	node := newFakeNode("n1")

	ep1 := &EndpointVeth{EndpointGeneric: EndpointGeneric{Node: node, IfaceName: "eth1"}}
	ep2 := &EndpointVeth{EndpointGeneric: EndpointGeneric{Node: node, IfaceName: "eth1"}}

	_ = node.AddEndpoint(ep1)
	_ = node.AddEndpoint(ep2)

	if err := CheckEndpointUniqueness(ep1); err == nil {
		t.Fatal("expected duplicate endpoint error, got nil")
	}

	ep2.IfaceName = "eth2"

	if err := CheckEndpointUniqueness(ep1); err != nil {
		t.Fatalf("expected no error for unique endpoints, got %v", err)
	}
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

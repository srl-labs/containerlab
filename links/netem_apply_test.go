package links

import (
	"context"
	"errors"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
)

type netemApplyNode struct {
	*fakeNode
	execCalls  int
	execErrors []error
}

func (n *netemApplyNode) ExecFunction(context.Context, func(ns.NetNS) error) error {
	var err error
	if n.execCalls < len(n.execErrors) {
		err = n.execErrors[n.execCalls]
	}
	n.execCalls++

	return err
}

func TestApplyLinkImpairmentsContinuesAfterEndpointError(t *testing.T) {
	execErr := errors.New("netlink failed")
	failing := &netemApplyNode{
		fakeNode:   newFakeNode("failing"),
		execErrors: []error{execErr},
	}
	working := &netemApplyNode{fakeNode: newFakeNode("working")}
	unimpaired := &netemApplyNode{fakeNode: newFakeNode("unimpaired")}

	endpoints := []Endpoint{
		NewEndpointVeth(NewEndpointGeneric(failing, "eth1", nil)),
		NewEndpointVeth(NewEndpointGeneric(working, "eth1", nil)),
		NewEndpointVeth(NewEndpointGeneric(unimpaired, "eth1", nil)),
	}
	for _, endpoint := range endpoints[:2] {
		endpoint.(*EndpointVeth).Netem = &Netem{Loss: 1}
	}

	loadCalls := 0
	err := applyLinkImpairments(context.Background(), endpoints, func(context.Context) {
		loadCalls++
	})
	if !errors.Is(err, execErr) {
		t.Fatalf("applyLinkImpairments() error = %v, want the endpoint error", err)
	}
	if failing.execCalls != 1 || working.execCalls != 1 || unimpaired.execCalls != 0 {
		t.Fatalf(
			"ExecFunction calls = failing %d, working %d, unimpaired %d; want 1, 1, 0",
			failing.execCalls,
			working.execCalls,
			unimpaired.execCalls,
		)
	}
	if loadCalls != 1 {
		t.Fatalf("kernel module load calls = %d, want 1", loadCalls)
	}
}

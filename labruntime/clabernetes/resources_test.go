package clabernetes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestConditionPendingReasonIdentifiesDirectHelper(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ conditionType, helper string }{
		{"Prepared", "planner"},
		{"ConnectivityReady", "clabwire"},
	} {
		for _, message := range []string{
			"required direct helper is not ready",
			"required direct helper has no container status",
		} {
			node := &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{"name": "r1", "namespace": "lab-ns"},
				"status": map[string]any{"conditions": []any{map[string]any{
					"type": tt.conditionType, "status": "False", "message": message,
				}}},
			}}
			got := conditionPendingReason(node, tt.conditionType)
			if !strings.Contains(got, "waiting for "+tt.helper+" container") ||
				!strings.Contains(got, "kubectl -n lab-ns logs deploy/r1 -c "+tt.helper) {
				t.Fatalf("helper diagnostic = %q", got)
			}
		}
	}
}

// resolvedTestLink builds a Link object the wait treats as resolved: an Accepted condition and
// both endpoints bound to nodes with identities.
func resolvedTestLink(name, namespace, owner string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Link",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
			"labels":    map[string]any{labelTopologyOwner: owner},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Accepted", "status": "True"},
			},
			"resolvedEndpoints": map[string]any{
				"endpointA": map[string]any{"nodeName": "r1", "uid": "uid-1"},
				"endpointB": map[string]any{"nodeName": "r2", "uid": "uid-2"},
			},
		},
	}}
}

// TestWaitPrimitiveLinksResolvedListsTopologyScopedLinks pins the list the wait poll performs:
// it must carry the topology-owner label selector, exactly like every other primitive list, so
// the poll stays proportional to the lab instead of the namespace in a shared namespace.
func TestWaitPrimitiveLinksResolvedListsTopologyScopedLinks(t *testing.T) {
	t.Parallel()

	const labName = "lab-a"

	link := resolvedTestLink("r1-eth1-r2-eth1", "lab-ns", labName)

	r := newTestRuntime(link)

	desired := &unstructured.Unstructured{}
	desired.SetName("r1-eth1-r2-eth1")

	if err := r.waitPrimitiveLinksResolved(
		context.Background(),
		"lab-ns",
		labName,
		[]*unstructured.Unstructured{desired},
		time.Second,
	); err != nil {
		t.Fatalf("the seeded resolved link must satisfy the wait: %s", err)
	}

	selectorFound := false
	for _, action := range r.client.(*dynamicfake.FakeDynamicClient).Actions() { //nolint:forcetypeassert // the test harness owns the client type
		if action.GetVerb() != "list" || action.GetResource().Resource != "links" {
			continue
		}

		listAction, ok := action.(k8stesting.ListAction)
		if !ok {
			t.Fatalf("list action has unexpected type: %T", action)
		}

		want := labels.Set{labelTopologyOwner: labName}.String()
		if got := listAction.GetListRestrictions().Labels.String(); got != want {
			t.Fatalf("link list selector = %q, want %q", got, want)
		}

		selectorFound = true
	}

	if !selectorFound {
		t.Fatal("the wait never listed the c9s links")
	}
}

// TestWaitPrimitiveLinksResolvedIgnoresForeignLinks proves the owner selector is load bearing:
// a foreign Link carrying the same name but no topology-owner label must never satisfy the wait
// for this lab's link.
func TestWaitPrimitiveLinksResolvedIgnoresForeignLinks(t *testing.T) {
	t.Parallel()

	const labName = "lab-a"

	// Same name, same namespace, fully resolved -- but owned by nobody. The client-side
	// name filter alone would have accepted it before the selector existed.
	foreign := resolvedTestLink("r1-eth1-r2-eth1", "lab-ns", "")
	foreign.SetLabels(nil)

	r := newTestRuntime(foreign)

	desired := &unstructured.Unstructured{}
	desired.SetName("r1-eth1-r2-eth1")

	err := r.waitPrimitiveLinksResolved(
		context.Background(),
		"lab-ns",
		labName,
		[]*unstructured.Unstructured{desired},
		time.Millisecond,
	)
	if err == nil {
		t.Fatal("a foreign link must not satisfy the wait for this lab's link")
	}

	if !strings.Contains(err.Error(), "timed out") ||
		!strings.Contains(err.Error(), "r1-eth1-r2-eth1 (not found)") {
		t.Fatalf("expected the timeout naming the pending link, got: %s", err)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the wait must translate the deadline into the timeout diagnostic: %s", err)
	}
}

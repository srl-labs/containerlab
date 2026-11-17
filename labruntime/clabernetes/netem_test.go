package clabernetes

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// netemTestRuntime is a lab "lab1" in namespace lab-ns with nodes r1 (a sanitized "R1") and r2,
// wired by the link r1-eth1-r2-eth1.
func netemTestRuntime(t *testing.T) *Runtime {
	t.Helper()

	objects := []*unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Link",
		"metadata": map[string]any{
			"name":      "r1-eth1-r2-eth1",
			"namespace": "lab-ns",
			"labels":    map[string]any{labelTopologyOwner: "lab1"},
		},
		"spec": map[string]any{
			"endpointA": map[string]any{"nodeName": "r1", "interfaceName": "eth1"},
			"endpointB": map[string]any{"nodeName": "r2", "interfaceName": "eth1"},
		},
	}}}

	pods := []k8sruntime.Object{}

	for name, original := range map[string]string{"r1": "R1", "r2": "r2"} {
		objects = append(objects, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       "Node",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "lab-ns",
				"uid":       "uid-" + name,
				"labels": map[string]any{
					labelTopologyOwner:    "lab1",
					labelOriginalNodeName: original,
				},
			},
			"spec":   map[string]any{"kind": "linux"},
			"status": map[string]any{"readiness": "ready"},
		}})
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lab1-" + name,
				Namespace: "lab-ns",
				Labels: map[string]string{
					labelApp:           clabernetesAppValue,
					labelTopologyOwner: "lab1",
					labelTopologyNode:  name,
				},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		})
	}

	r := newTestRuntimeWithKubeObjects(objects, pods)
	r.labNamespaceOverride = "lab-ns"

	return r
}

func TestSetImpairmentChangesTheLinkEndpoint(t *testing.T) {
	t.Parallel()

	r := netemTestRuntime(t)
	ctx := context.Background()

	err := r.SetImpairment(ctx, clablabruntime.ImpairmentRequest{
		Name: "lab1", Node: "R1", Interface: "eth1",
		Impairment: &clablabruntime.Impairment{
			Delay: 50 * time.Millisecond, Jitter: 5 * time.Millisecond, Loss: 1, Rate: 10000,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	link, err := r.client.Resource(linkGVR).Namespace("lab-ns").
		Get(ctx, "r1-eth1-r2-eth1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// JSON numbers decode by their spelling, so the impairment is compared as JSON
	netem, _, _ := unstructured.NestedMap(link.Object, "spec", "endpointA", "netem")
	raw, err := json.Marshal(netem)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"delay":"50ms","jitter":"5ms","loss":1,"rate":10000}`; string(raw) != want {
		t.Fatalf("endpointA netem = %s, want %s", raw, want)
	}

	if _, found, _ := unstructured.NestedMap(link.Object, "spec", "endpointB", "netem"); found {
		t.Fatal("the impairment of r1 reached the endpoint of r2")
	}

	nodes, err := r.Impairments(ctx, clablabruntime.NodeRequest{Name: "lab1"})
	if err != nil {
		t.Fatal(err)
	}

	wantNodes := []clablabruntime.NodeImpairments{
		{Node: "R1", Impairments: []clabtypes.ImpairmentData{{
			Interface: "eth1", Delay: "50ms", Jitter: "5ms", PacketLoss: 1, Rate: 10000,
		}}},
		{Node: "r2"},
	}
	if !reflect.DeepEqual(nodes, wantNodes) {
		t.Fatalf("Impairments() = %#v, want %#v", nodes, wantNodes)
	}

	// Without an impairment the request resets the interface.
	err = r.SetImpairment(ctx, clablabruntime.ImpairmentRequest{
		Name: "lab1", Node: "R1", Interface: "eth1",
	})
	if err != nil {
		t.Fatal(err)
	}

	link, err = r.client.Resource(linkGVR).Namespace("lab-ns").
		Get(ctx, "r1-eth1-r2-eth1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if _, found, _ := unstructured.NestedFieldNoCopy(
		link.Object,
		"spec",
		"endpointA",
		"netem",
	); found {
		t.Fatalf("reset left the impairment on the link: %#v", link.Object["spec"])
	}
}

func TestSetImpairmentRejectsAnInterfaceOffTheLabLinks(t *testing.T) {
	t.Parallel()

	err := netemTestRuntime(t).SetImpairment(context.Background(),
		clablabruntime.ImpairmentRequest{
			Name: "lab1", Node: "r2", Interface: "eth9",
			Impairment: &clablabruntime.Impairment{Delay: time.Millisecond},
		})
	if err == nil ||
		!strings.Contains(err.Error(), `interface "eth9" of node "r2" is not on a link`) {
		t.Fatalf("SetImpairment() error = %v", err)
	}
}

func TestSetImpairmentRejectsATopologyOwnedLab(t *testing.T) {
	t.Parallel()

	r := netemTestRuntime(t)
	topology := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       "Topology",
		"metadata":   map[string]any{"name": "lab1", "namespace": "lab-ns"},
	}}
	if _, err := r.client.Resource(topologyGVR).Namespace("lab-ns").Create(
		context.Background(), topology, metav1.CreateOptions{},
	); err != nil {
		t.Fatal(err)
	}

	err := r.SetImpairment(context.Background(), clablabruntime.ImpairmentRequest{
		Name: "lab1", Node: "r1", Interface: "eth1",
		Impairment: &clablabruntime.Impairment{Delay: time.Millisecond},
	})
	if err == nil || !strings.Contains(err.Error(), "--no-topology-cr") {
		t.Fatalf("SetImpairment() on a Topology-owned lab error = %v", err)
	}
}

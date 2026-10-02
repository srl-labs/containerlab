package clabernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDeleteManagedLabNamespacePreservesSharedOrOverriddenNamespace(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, override, resourceKind string
		gvr                          schema.GroupVersionResource
	}{
		{name: "explicit canonical namespace", override: "c9s-lab1"},
		{name: "other topology", resourceKind: "Topology", gvr: topologyGVR},
		{name: "other primitive lab", resourceKind: "Node", gvr: nodeGVR},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRuntime()
			r.labNamespaceOverride = tt.override
			if _, err := r.kubeClient.CoreV1().Namespaces().Create(context.Background(),
				labNamespaceObject("lab1", "c9s-lab1"), metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}
			if tt.resourceKind != "" {
				other := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": c9sAPIVersion, "kind": tt.resourceKind,
					"metadata": map[string]any{
						"name": "other", "namespace": "c9s-lab1",
						"labels": map[string]any{labelTopologyOwner: "lab2"},
					},
				}}
				if _, err := r.client.Resource(tt.gvr).Namespace("c9s-lab1").Create(
					context.Background(), other, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			deleted, err := r.deleteManagedLabNamespace(context.Background(), "lab1", "c9s-lab1")
			if err != nil || deleted {
				t.Fatalf("shared/overridden namespace deletion = %v, %v", deleted, err)
			}
			if _, err := r.kubeClient.CoreV1().Namespaces().Get(context.Background(), "c9s-lab1",
				metav1.GetOptions{}); err != nil {
				t.Fatalf("shared/overridden namespace was lost: %v", err)
			}
		})
	}
}

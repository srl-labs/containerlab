package clabernetes

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

package labruntime

import (
	"strings"
	"testing"
)

func TestNormalizeExposeType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},
		{input: "ClusterIP", want: ExposeTypeClusterIP},
		{input: "clusterip", want: ExposeTypeClusterIP},
		{input: "Headless", want: ExposeTypeHeadless},
		{input: "headless", want: ExposeTypeHeadless},
		{input: "LoadBalancer", want: ExposeTypeLoadBalancer},
		{input: "loadbalancer", want: ExposeTypeLoadBalancer},
		{input: "None", want: ExposeTypeNone},
		{input: "none", want: ExposeTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeExposeType(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeExposeType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	_, err := NormalizeExposeType("NodePort")
	if err == nil || !strings.Contains(err.Error(), "ClusterIP") {
		t.Fatalf("unexpected invalid expose type error: %v", err)
	}
}

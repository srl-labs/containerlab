package cmd

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
)

func TestPlaceholderNodeKind(t *testing.T) {
	tests := map[clablinks.LinkEndpointType]string{
		clablinks.LinkEndpointTypeHost:     "host",
		clablinks.LinkEndpointTypeBridge:   "bridge",
		clablinks.LinkEndpointTypeBridgeNS: "bridge",
		clablinks.LinkEndpointTypeVeth:     "ext-container",
		"anything-else":                    "ext-container",
	}

	for epType, want := range tests {
		if got := placeholderNodeKind(epType); got != want {
			t.Errorf("placeholderNodeKind(%q) = %q, want %q", epType, got, want)
		}
	}
}

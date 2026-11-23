package bridge

import (
	"strings"
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestBridgeValidateContainerModeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		node    string
		network string
		wantErr string
	}{
		{
			name:    "well-formed bridge name passes",
			node:    "br01|bp1",
			network: "container:bp1",
		},
		{
			name:    "missing separator is rejected",
			node:    "br01",
			network: "container:bp1",
			wantErr: "requires the name pattern",
		},
		{
			name:    "empty bridge device part is rejected",
			node:    "|bp1",
			network: "container:bp1",
			wantErr: "requires the name pattern",
		},
		{
			name:    "extra separator is rejected",
			node:    "br|01|bp1",
			network: "container:bp1",
			wantErr: "bp1 != 01|bp1",
		},
		{
			name:    "mismatched parent suffix is rejected",
			node:    "br01|bp2",
			network: "container:bp1",
			wantErr: "bp1 != bp2",
		},
		{
			name:    "invalid network mode form is rejected",
			node:    "br01|bp1",
			network: "container:",
			wantErr: "names no container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := &bridge{}
			b.Cfg = &clabtypes.NodeConfig{
				ShortName:   tt.node,
				NetworkMode: tt.network,
			}

			err := b.validateContainerModeName()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error message %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

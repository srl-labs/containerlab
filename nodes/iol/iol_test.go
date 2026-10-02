package cisco_iol

import (
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestL2Detection(t *testing.T) {
	tests := []struct {
		nodeType, image string
		want            bool
	}{
		{"", "vrnetlab/cisco_iol:l2-17.12.01", true},
		{"", "registry/cisco_IOL-L2:17.12", true},
		{"", "vrnetlab/cisco_iol:17.12.01", false},
		{"", "cl2.example.com/cisco_iol:17.12", false},
		{"iol", "vrnetlab/cisco_iol:l2-17.12.01", false},
		{"l2", "vrnetlab/cisco_iol:17.12.01", true},
	}

	for _, tt := range tests {
		n := &iol{}
		cfg := &clabtypes.NodeConfig{NodeType: tt.nodeType, Image: tt.image}
		if err := n.Init(cfg); err != nil {
			t.Fatalf("Init(%q, %q): %v", tt.nodeType, tt.image, err)
		}
		if n.isL2Node != tt.want {
			t.Errorf("type=%q image=%q: isL2Node=%v, want %v", tt.nodeType, tt.image, n.isL2Node, tt.want)
		}
	}
}

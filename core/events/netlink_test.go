package events

import (
	"maps"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func TestOverlayToolsInterfaceLeavesAttributesWhenNotStitched(t *testing.T) {
	base := map[string]string{
		"ifname": "eth1",
		"alias":  "e1-1",
		"type":   "veth",
	}

	tests := map[string]*clabruntime.GenericContainer{
		"no labels": {},
		"lab only":  {Labels: map[string]string{clabconstants.Containerlab: "lab"}},
		"node only": {Labels: map[string]string{clabconstants.NodeName: "n1"}},
		// lab+node set, but no stitch interface exists, so attributes stay put.
		"no stitch iface": {Labels: map[string]string{
			clabconstants.Containerlab: "no-such-lab",
			clabconstants.NodeName:     "no-such-node",
		}},
	}

	for name, container := range tests {
		t.Run(name, func(t *testing.T) {
			attrs := maps.Clone(base)
			overlayToolsInterface(container, attrs)

			if !maps.Equal(attrs, base) {
				t.Fatalf("attributes changed unexpectedly: got %v want %v", attrs, base)
			}
		})
	}
}

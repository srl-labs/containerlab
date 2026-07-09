package core

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// TestApplyNodeDefaultLinkTypes verifies that a brief link adopts the veth-stitch
// type when a touched node's kind declares it via PlatformAttrs.DefaultLinkType,
// and is left alone otherwise (both-veth, or an explicitly typed link).
func TestApplyNodeDefaultLinkTypes(t *testing.T) {
	reg := clabnodes.NewNodeRegistry()
	initFn := func() clabnodes.Node { return nil }

	if err := reg.Register([]string{"kveth"}, initFn, nil); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register([]string{"kstitch"}, initFn,
		clabnodes.NewNodeRegistryEntryAttributes(nil, nil,
			&clabnodes.PlatformAttrs{DefaultLinkType: clablinks.LinkTypeVethStitch})); err != nil {
		t.Fatal(err)
	}

	briefVeth := func() *clablinks.LinkVEthRaw {
		return &clablinks.LinkVEthRaw{
			Endpoints: []*clablinks.EndpointRaw{
				{Node: "a", Iface: "eth1"},
				{Node: "b", Iface: "eth1"},
			},
		}
	}

	tests := []struct {
		name         string
		linkType     string
		kindA, kindB string
		wantStitch   bool
	}{
		{"both veth stays veth", string(clablinks.LinkTypeBrief), "kveth", "kveth", false},
		{"one stitch node upgrades", string(clablinks.LinkTypeBrief), "kveth", "kstitch", true},
		{"both stitch nodes upgrade", string(clablinks.LinkTypeBrief), "kstitch", "kstitch", true},
		{"explicit type is left untouched", string(clablinks.LinkTypeVEth), "kveth", "kstitch", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld := &clablinks.LinkDefinition{Type: tt.linkType, Link: briefVeth()}
			c := &CLab{
				Reg: reg,
				Config: &Config{
					Topology: &clabtypes.Topology{
						Nodes: map[string]*clabtypes.NodeDefinition{
							"a": {Kind: tt.kindA},
							"b": {Kind: tt.kindB},
						},
						Links: []*clablinks.LinkDefinition{ld},
					},
				},
			}

			c.applyNodeDefaultLinkTypes()

			_, gotStitch := ld.Link.(*clablinks.LinkVEthStitchedRaw)
			if gotStitch != tt.wantStitch {
				t.Fatalf("stitch=%v, want %v (got link %T)", gotStitch, tt.wantStitch, ld.Link)
			}
		})
	}
}

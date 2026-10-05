package links

import "testing"

func TestResolveVxlanStitchedSanitizesHostIfaceNames(t *testing.T) {
	r := &LinkVxlanRaw{
		LinkType: LinkTypeVxlanStitch,
		Remote:   "169.254.1.2",
		VNI:      7,
		Endpoint: EndpointRaw{Node: "d1", Iface: "1/1/c3/1"},
	}

	params := &ResolveParams{
		Nodes: map[string]Node{
			"d1":   newFakeNode("d1"),
			"host": GetHostLinkNode(),
		},
	}

	vxs, err := r.ResolveStitched(params)
	if err != nil {
		t.Fatalf("ResolveStitched() error = %v", err)
	}

	if got, want := vxs.vxlanLink.localEndpoint.GetIfaceName(), "vx-d1_1-1-c3-1"; got != want {
		t.Errorf("vxlan host iface = %q, want %q", got, want)
	}
	if got, want := vxs.vethStitchEp.GetIfaceName(), "ve-d1_1-1-c3-1"; got != want {
		t.Errorf("veth host iface = %q, want %q", got, want)
	}
}

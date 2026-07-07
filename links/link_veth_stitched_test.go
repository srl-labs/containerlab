package links

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func vethStitchResolveParams() *ResolveParams {
	return &ResolveParams{
		LabName: "mylab",
		Nodes: map[string]Node{
			"pe01": newFakeNode("pe01"),
			"pe02": newFakeNode("pe02"),
		},
	}
}

func TestVethStitchedUnmarshalYAML(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
type: veth-stitch
endpoints:
  - node: pe01
    interface: 1/1/c3/1
  - node: pe02
    interface: 1/1/c3/1
`), &linkDef)
	if err != nil {
		t.Fatalf("yaml.UnmarshalStrict() error = %v", err)
	}

	raw, ok := linkDef.Link.(*LinkVEthStitchedRaw)
	if !ok {
		t.Fatalf("link raw type = %T, want *LinkVEthStitchedRaw", linkDef.Link)
	}
	if raw.GetType() != LinkTypeVethStitch {
		t.Fatalf("GetType() = %q, want %q", raw.GetType(), LinkTypeVethStitch)
	}
	if len(raw.Endpoints) != 2 {
		t.Fatalf("endpoints = %d, want 2", len(raw.Endpoints))
	}
}

func TestVethStitchedUnmarshalBriefYAML(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
type: veth-stitch
endpoints: ["sros1:1/1/c1/1", "sros:1/1/c1/1"]
`), &linkDef)
	if err != nil {
		t.Fatalf("yaml.UnmarshalStrict() error = %v", err)
	}

	raw, ok := linkDef.Link.(*LinkVEthStitchedRaw)
	if !ok {
		t.Fatalf("link raw type = %T, want *LinkVEthStitchedRaw", linkDef.Link)
	}

	want := []*EndpointRaw{
		{Node: "sros1", Iface: "1/1/c1/1"},
		{Node: "sros", Iface: "1/1/c1/1"},
	}
	for i, w := range want {
		if raw.Endpoints[i].Node != w.Node || raw.Endpoints[i].Iface != w.Iface {
			t.Fatalf("endpoint[%d] = %s:%s, want %s:%s",
				i, raw.Endpoints[i].Node, raw.Endpoints[i].Iface, w.Node, w.Iface)
		}
	}
}

func TestResolveVethStitched(t *testing.T) {
	r := &LinkVEthStitchedRaw{
		Endpoints: []*EndpointRaw{
			{Node: "pe01", Iface: "1/1/c3/1"},
			{Node: "pe02", Iface: "1/1/c3/1"},
		},
	}

	l, err := r.Resolve(vethStitchResolveParams())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	stitched, ok := l.(*LinkVEthStitched)
	if !ok {
		t.Fatalf("resolved link type = %T, want *LinkVEthStitched", l)
	}

	// the in-netns ends are named after the facing node so they are intuitive to
	// target with tc/tcpdump.
	if got := stitched.epA.GetIfaceName(); got != "pe01" {
		t.Fatalf("epA iface = %q, want %q", got, "pe01")
	}
	if got := stitched.epB.GetIfaceName(); got != "pe02" {
		t.Fatalf("epB iface = %q, want %q", got, "pe02")
	}

	// each segment: [ real node endpoint, in-netns far endpoint ]
	if got := stitched.segA.Endpoints[0].GetNode().GetShortName(); got != "pe01" {
		t.Fatalf("segA node endpoint owner = %q, want %q", got, "pe01")
	}
	if !stitched.epA.IsNodeless() {
		t.Fatalf("epA should be nodeless so the veth deploy pushes it into the netns immediately")
	}

	// the logical endpoints are the two node-side ones.
	eps := stitched.GetEndpoints()
	if len(eps) != 2 ||
		eps[0].GetNode().GetShortName() != "pe01" ||
		eps[1].GetNode().GetShortName() != "pe02" {
		t.Fatalf("GetEndpoints() = %v, want [pe01 pe02] node-side endpoints", eps)
	}

	if !strings.HasPrefix(stitched.node.netnsName, "clab-mylab-vstitch-") {
		t.Fatalf("netns name = %q, want clab-mylab-vstitch- prefix", stitched.node.netnsName)
	}
}

func TestResolveVethStitchedFilteredOut(t *testing.T) {
	params := vethStitchResolveParams()
	params.NodesFilter = []string{"pe01"} // pe02 not selected -> link is skipped

	r := &LinkVEthStitchedRaw{
		Endpoints: []*EndpointRaw{
			{Node: "pe01", Iface: "1/1/c3/1"},
			{Node: "pe02", Iface: "1/1/c3/1"},
		},
	}

	l, err := r.Resolve(params)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if l != nil {
		t.Fatalf("Resolve() = %v, want nil (filtered out)", l)
	}
}

func TestValidateVEthStitched(t *testing.T) {
	tests := []struct {
		name    string
		raw     *LinkVEthStitchedRaw
		wantErr bool
	}{
		{
			name: "valid",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe02", Iface: "e1"}},
			},
		},
		{
			name: "not two endpoints",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{{Node: "pe01", Iface: "e1"}},
			},
			wantErr: true,
		},
		{
			name: "self link",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe01", Iface: "e2"}},
			},
			wantErr: true,
		},
		{
			name: "node name too long for interface",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{
					{Node: "this-node-name-is-too-long", Iface: "e1"},
					{Node: "pe02", Iface: "e1"},
				},
			},
			wantErr: true,
		},
		{
			name: "endpoint carries mac",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{
					{Node: "pe01", Iface: "e1", MAC: "00:11:22:33:44:55"},
					{Node: "pe02", Iface: "e1"},
				},
			},
			wantErr: true,
		},
		{
			name: "link carries ipv4",
			raw: &LinkVEthStitchedRaw{
				LinkCommonParams: LinkCommonParams{IPv4: []string{"10.0.0.0/31"}},
				Endpoints:        []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe02", Iface: "e1"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVEthStitched(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateVEthStitched() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVethStitchNetnsName(t *testing.T) {
	eps := []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe02", Iface: "e1"}}

	name := VEthStitchNetnsName("lab", eps)

	// deterministic
	if name != VEthStitchNetnsName("lab", eps) {
		t.Fatalf("netns name is not deterministic")
	}
	// lab-scoped
	if !strings.HasPrefix(name, "clab-lab-vstitch-") {
		t.Fatalf("netns name = %q, want clab-lab-vstitch- prefix", name)
	}
	// differs by lab
	if name == VEthStitchNetnsName("other", eps) {
		t.Fatalf("netns name should differ across labs")
	}
	// differs by endpoints
	other := []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe03", Iface: "e1"}}
	if name == VEthStitchNetnsName("lab", other) {
		t.Fatalf("netns name should differ across endpoint sets")
	}
}

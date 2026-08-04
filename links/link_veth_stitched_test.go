package links

import (
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

	// the root-ns far ends carry a deterministic hash name (the readable name
	// lives in the clab-s- altname added at PostDeploy).
	if got, want := stitched.epA.GetIfaceName(), stitchFarEndName("mylab", "pe01", "1/1/c3/1"); got != want {
		t.Fatalf("epA far-end name = %q, want %q", got, want)
	}
	if got, want := stitched.epB.GetIfaceName(), stitchFarEndName("mylab", "pe02", "1/1/c3/1"); got != want {
		t.Fatalf("epB far-end name = %q, want %q", got, want)
	}

	// each segment: [ real node endpoint, in-netns far endpoint ]
	if got := stitched.segA.Endpoints[0].GetNode().GetShortName(); got != "pe01" {
		t.Fatalf("segA node endpoint owner = %q, want %q", got, "pe01")
	}
	if !stitched.epA.IsNodeless() {
		t.Fatalf("epA should be nodeless so the veth deploy pushes it into the root ns immediately")
	}

	// the logical endpoints are the two node-side ones.
	eps := stitched.GetEndpoints()
	if len(eps) != 2 ||
		eps[0].GetNode().GetShortName() != "pe01" ||
		eps[1].GetNode().GetShortName() != "pe02" {
		t.Fatalf("GetEndpoints() = %v, want [pe01 pe02] node-side endpoints", eps)
	}

	// far ends live on the host (root-ns) node
	if got := stitched.epA.GetNode().GetShortName(); got != "host" {
		t.Fatalf("epA far end node = %q, want host (root ns)", got)
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

func TestNewVEthStitchedRawFromVEth(t *testing.T) {
	v := &LinkVEthRaw{
		LinkCommonParams: LinkCommonParams{MTU: 1400},
		Endpoints: []*EndpointRaw{
			{Node: "a", Iface: "e1"},
			{Node: "b", Iface: "e2"},
		},
	}

	s := NewVEthStitchedRawFromVEth(v)

	if s.GetType() != LinkTypeVethStitch {
		t.Fatalf("type = %q, want %q", s.GetType(), LinkTypeVethStitch)
	}
	if s.MTU != 1400 {
		t.Fatalf("MTU = %d, want 1400", s.MTU)
	}
	if len(s.Endpoints) != 2 || s.Endpoints[0].Node != "a" || s.Endpoints[1].Node != "b" {
		t.Fatalf("endpoints not carried over: %+v", s.Endpoints)
	}
}

package links

import (
	"context"
	"sync"
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
		LinkCommonParams: LinkCommonParams{
			Labels: map[string]string{"role": "dataplane"},
			IPv4:   []string{"10.0.0.1/31", "10.0.0.0/31"},
		},
		Endpoints: []*EndpointRaw{
			{
				Node:  "pe01",
				Iface: "1/1/c3/1",
				MAC:   "02:00:00:00:00:01",
				IPv4:  "10.0.0.1/31",
			},
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
	if got, want := stitched.epA.GetIfaceName(), stitchFarEndName(
		"mylab",
		"pe01",
		"1/1/c3/1",
	); got != want {
		t.Fatalf("epA far-end name = %q, want %q", got, want)
	}
	if got, want := stitched.epB.GetIfaceName(), stitchFarEndName(
		"mylab",
		"pe02",
		"1/1/c3/1",
	); got != want {
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
	if got := stitched.GetEndpoints()[0].GetLink(); got != stitched {
		t.Fatalf("node endpoint link = %T, want *LinkVEthStitched", got)
	}
	if got := stitched.epA.GetLink(); got != stitched.segA {
		t.Fatalf("far endpoint link = %T, want *LinkVEth", got)
	}

	runtimeEndpoints := stitched.GetRuntimeEndpoints()
	if len(runtimeEndpoints) != 4 {
		t.Fatalf("GetRuntimeEndpoints() returned %d endpoints, want 4", len(runtimeEndpoints))
	}
	if runtimeEndpoints[2] != stitched.epA || runtimeEndpoints[3] != stitched.epB {
		t.Fatalf("GetRuntimeEndpoints() does not include both host far ends")
	}

	if got := stitched.Labels["role"]; got != "dataplane" {
		t.Fatalf("resolved link label = %q, want dataplane", got)
	}
	if got := stitched.GetEndpoints()[0].GetIPv4Addr().String(); got != "10.0.0.1/31" {
		t.Fatalf("node endpoint IPv4 = %q, want 10.0.0.1/31", got)
	}
	if got := stitched.GetEndpoints()[0].GetMac().String(); got != "02:00:00:00:00:01" {
		t.Fatalf("node endpoint MAC = %q, want 02:00:00:00:00:01", got)
	}
}

func TestVethStitchedRemoveConcurrent(t *testing.T) {
	l, err := (&LinkVEthStitchedRaw{
		Endpoints: []*EndpointRaw{
			{Node: "pe01", Iface: "e1"},
			{Node: "pe02", Iface: "e1"},
		},
	}).Resolve(vethStitchResolveParams())
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	stitched := l.(*LinkVEthStitched)
	stitched.segA.DeploymentState = LinkDeploymentStateRemoved
	stitched.segB.DeploymentState = LinkDeploymentStateRemoved

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- stitched.Remove(context.Background())
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
	}
	if stitched.DeploymentState != LinkDeploymentStateRemoved {
		t.Fatalf("DeploymentState = %d, want removed", stitched.DeploymentState)
	}
}

func TestToolsInterfaceGuardsEmptyIdentity(t *testing.T) {
	for name, args := range map[string][3]string{
		"empty lab":   {"", "n1", "e1"},
		"empty node":  {"lab", "", "e1"},
		"empty iface": {"lab", "n1", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if link, ok := ToolsInterface(args[0], args[1], args[2]); ok || link != nil {
				t.Fatalf("expected no tools interface for %v, got (%v, %v)", args, link, ok)
			}
		})
	}
}

func TestFilteredVethStitchAltNames(t *testing.T) {
	got := filteredVethStitchAltNames(
		"lab",
		[]string{"n1"},
		[]*EndpointRaw{
			{Node: "n1", Iface: "e2"},
			{Node: "n2", Iface: "e1"},
			{Node: "n1", Iface: "e1"},
			{Node: "n1", Iface: "e1"},
		},
	)
	want := []string{
		StitchAltName("lab", "n1", "e1"),
		StitchAltName("lab", "n1", "e2"),
	}

	if len(got) != len(want) {
		t.Fatalf("filteredVethStitchAltNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filteredVethStitchAltNames()[%d] = %q, want %q", i, got[i], want[i])
		}
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
			name: "endpoint addressing is supported",
			raw: &LinkVEthStitchedRaw{
				Endpoints: []*EndpointRaw{
					{
						Node:  "pe01",
						Iface: "e1",
						MAC:   "00:11:22:33:44:55",
						IPv4:  "10.0.0.1/31",
						IPv6:  "2001:db8::1/127",
					},
					{Node: "pe02", Iface: "e1"},
				},
			},
		},
		{
			name: "link addressing is supported",
			raw: &LinkVEthStitchedRaw{
				LinkCommonParams: LinkCommonParams{
					IPv4: []string{"10.0.0.0/31"},
					IPv6: []string{"2001:db8::/127"},
				},
				Endpoints: []*EndpointRaw{{Node: "pe01", Iface: "e1"}, {Node: "pe02", Iface: "e1"}},
			},
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

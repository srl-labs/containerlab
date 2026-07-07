package links

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

func TestExpandImpairmentsFromExtendedVethYAML(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
type: veth
endpoints:
  - node: pe01
    interface: 1/1/c3/1
  - node: pe02
    interface: 1/1/c3/1
impairment:
  mode: bridge
  name: link-0102--01
`), &linkDef)
	if err != nil {
		t.Fatalf("yaml.UnmarshalStrict() error = %v", err)
	}

	expansion, err := ExpandImpairments([]*LinkDefinition{&linkDef}, nil)
	if err != nil {
		t.Fatalf("ExpandImpairments() error = %v", err)
	}

	if len(expansion.BridgeNodes) != 1 {
		t.Fatalf("generated bridge nodes = %d, want 1", len(expansion.BridgeNodes))
	}
	if diff := cmp.Diff(&ImpairmentBridgeNode{
		Name:          "link-0102--01",
		Image:         DefaultImpairmentBridgeImage,
		OriginalNodes: []string{"pe01", "pe02"},
	}, expansion.BridgeNodes[0]); diff != "" {
		t.Fatalf("bridge node mismatch (-want +got):\n%s", diff)
	}

	if len(expansion.Links) != 2 {
		t.Fatalf("expanded links = %d, want 2", len(expansion.Links))
	}
	assertVethRawEndpoints(t, expansion.Links[0], []*EndpointRaw{
		{Node: "pe01", Iface: "1/1/c3/1"},
		{Node: "link-0102--01", Iface: "eth1"},
	})
	assertVethRawEndpoints(t, expansion.Links[1], []*EndpointRaw{
		{Node: "link-0102--01", Iface: "eth2"},
		{Node: "pe02", Iface: "1/1/c3/1"},
	})
}

func TestExpandImpairmentsRejectsBriefYAML(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
endpoints: ["pe01:1/1/c3/1", "pe02:1/1/c3/1"]
impairment:
  mode: bridge
  name: link-0102--01
`), &linkDef)
	if err == nil {
		t.Fatalf("yaml.UnmarshalStrict() error = nil, want unsupported brief impairment error")
	}
}

func TestExpandImpairmentsAutoExpandsRegularVethWhenRequired(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
endpoints: ["pe01:1/1/c3/1", "pe02:1/1/c3/1"]
`), &linkDef)
	if err != nil {
		t.Fatalf("yaml.UnmarshalStrict() error = %v", err)
	}

	expansion, err := ExpandImpairments(
		[]*LinkDefinition{&linkDef},
		func(endpointNodes []string) bool {
			return endpointNodes[0] == "pe01" || endpointNodes[1] == "pe01"
		},
	)
	if err != nil {
		t.Fatalf("ExpandImpairments() error = %v", err)
	}

	if len(expansion.BridgeNodes) != 1 {
		t.Fatalf("generated bridge nodes = %d, want 1", len(expansion.BridgeNodes))
	}
	if diff := cmp.Diff(&ImpairmentBridgeNode{
		Name:          "impairment-bridge-01",
		Image:         DefaultImpairmentBridgeImage,
		OriginalNodes: []string{"pe01", "pe02"},
	}, expansion.BridgeNodes[0]); diff != "" {
		t.Fatalf("bridge node mismatch (-want +got):\n%s", diff)
	}

	assertVethRawEndpoints(t, expansion.Links[0], []*EndpointRaw{
		{Node: "pe01", Iface: "1/1/c3/1"},
		{Node: "impairment-bridge-01", Iface: "eth1"},
	})
	assertVethRawEndpoints(t, expansion.Links[1], []*EndpointRaw{
		{Node: "impairment-bridge-01", Iface: "eth2"},
		{Node: "pe02", Iface: "1/1/c3/1"},
	})
}

func TestExpandImpairmentsKeepsRegularVethDirectWhenBridgeIsNotRequired(t *testing.T) {
	var linkDef LinkDefinition

	err := yaml.UnmarshalStrict([]byte(`
endpoints: ["pe01:eth1", "pe02:eth1"]
`), &linkDef)
	if err != nil {
		t.Fatalf("yaml.UnmarshalStrict() error = %v", err)
	}

	expansion, err := ExpandImpairments(
		[]*LinkDefinition{&linkDef},
		func([]string) bool { return false },
	)
	if err != nil {
		t.Fatalf("ExpandImpairments() error = %v", err)
	}
	if len(expansion.BridgeNodes) != 0 {
		t.Fatalf("generated bridge nodes = %d, want 0", len(expansion.BridgeNodes))
	}
	if diff := cmp.Diff([]*LinkDefinition{&linkDef}, expansion.Links); diff != "" {
		t.Fatalf("links mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandImpairmentsRejectsUnsupportedMode(t *testing.T) {
	linkDef := &LinkDefinition{
		Type: string(LinkTypeVEth),
		Link: &LinkVEthRaw{
			Endpoints: []*EndpointRaw{
				{Node: "pe01", Iface: "eth1"},
				{Node: "pe02", Iface: "eth1"},
			},
			Impairment: &LinkImpairment{
				Mode: "direct",
				Name: "link-0102--01",
			},
		},
	}

	if _, err := ExpandImpairments([]*LinkDefinition{linkDef}, nil); err == nil {
		t.Fatalf("ExpandImpairments() error = nil, want unsupported mode error")
	}
}

func TestExpandImpairmentsRejectsDuplicateBridgeNode(t *testing.T) {
	linkDefs := []*LinkDefinition{
		{
			Type: string(LinkTypeVEth),
			Link: &LinkVEthRaw{
				Endpoints: []*EndpointRaw{
					{Node: "pe01", Iface: "eth1"},
					{Node: "pe02", Iface: "eth1"},
				},
				Impairment: &LinkImpairment{Mode: "bridge", Name: "link-0102--01"},
			},
		},
		{
			Type: string(LinkTypeVEth),
			Link: &LinkVEthRaw{
				Endpoints: []*EndpointRaw{
					{Node: "pe03", Iface: "eth1"},
					{Node: "pe04", Iface: "eth1"},
				},
				Impairment: &LinkImpairment{Mode: "bridge", Name: "link-0102--01"},
			},
		},
	}

	if _, err := ExpandImpairments(linkDefs, nil); err == nil {
		t.Fatalf("ExpandImpairments() error = nil, want duplicate bridge node error")
	}
}

func assertVethRawEndpoints(t *testing.T, linkDef *LinkDefinition, want []*EndpointRaw) {
	t.Helper()

	raw, ok := linkDef.Link.(*LinkVEthRaw)
	if !ok {
		t.Fatalf("link raw type = %T, want *links.LinkVEthRaw", linkDef.Link)
	}

	if diff := cmp.Diff(want, raw.Endpoints); diff != "" {
		t.Fatalf("expanded endpoints mismatch (-want +got):\n%s", diff)
	}
}

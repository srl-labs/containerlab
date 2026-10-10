// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type testLogger struct {
	warnings []string
	errors   []string
}

func (l *testLogger) Debugf(string, ...any) {}

func (l *testLogger) Warnf(f string, a ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(f, a...))
}

func (l *testLogger) Errorf(f string, a ...any) {
	l.errors = append(l.errors, fmt.Sprintf(f, a...))
}

// compileForTest compiles a definition and fails the test on any compile error, returning the
// compiled topology and the unstructured primitives.
func compileForTest(
	t *testing.T,
	definition string,
) (*CompiledTopology, []unstructured.Unstructured, []unstructured.Unstructured, []unstructured.Unstructured) {
	t.Helper()

	input := &Input{Name: "lab", Namespace: "ns", Definition: definition}
	compiled, err := Compile(&testLogger{}, input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	nodes, links, profiles, err := CompileTopology(input, compiled)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	return compiled, nodes, links, profiles
}

func nodeByName(nodes []unstructured.Unstructured, name string) (map[string]any, bool) {
	for i := range nodes {
		if nodes[i].GetName() == name {
			spec, ok := nodes[i].Object["spec"].(map[string]any)

			return spec, ok
		}
	}

	return nil, false
}

func TestCompileFlattensInheritanceIntoNodes(t *testing.T) {
	compiled, nodes, links, profiles := compileForTest(t, `
name: lab
topology:
  defaults:
    kind: linux
    image: defaults-image
  kinds:
    linux:
      image: kind-image
      env: { tier: kind }
  groups:
    leafs:
      image: group-image
      labels: { tier: group }
  nodes:
    node1:
      group: leafs
      labels: { tier: node }
      env: { tier: node }
    node2:
      kind: linux
`)
	if len(compiled.Nodes) != 2 {
		t.Fatalf("compiled nodes = %d, want 2", len(compiled.Nodes))
	}

	node1 := compiled.Nodes["node1"]
	if node1.Kind != "linux" {
		t.Fatalf("node1 kind = %q, want linux", node1.Kind)
	}

	// The engine resolves every field through containerlab's own accessors, so the group layer
	// is more specific than the kind and defaults layers exactly as a containerlab deploy
	// resolves it.
	if node1.Image != "group-image" {
		t.Fatalf("node1 image = %q, want group-image", node1.Image)
	}

	if got := node1.Labels["tier"]; got != "node" {
		t.Fatalf("node1 label tier = %q, want node (most specific wins)", got)
	}

	node2 := compiled.Nodes["node2"]
	if node2.Image != "kind-image" {
		t.Fatalf("node2 image = %q, want kind-image (kind beats defaults)", node2.Image)
	}

	if len(nodes) != 2 || len(links) != 0 || len(profiles) != 1 {
		t.Fatalf(
			"rendered primitives: nodes=%d links=%d profiles=%d",
			len(nodes),
			len(links),
			len(profiles),
		)
	}
}

func TestCompileRejectsUnknownAndRejectedFields(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
topology:
  nodes:
    node1:
      kind: linux
      image: img
      not-a-clab-field: 1
      stages:
        ready:
          wait-for: [other]
`})
	if err == nil {
		t.Fatal("expected a compile error, got none")
	}

	// The stages field is a containerlab generic field the direct runtime cannot realize, so
	// the compile rejects it by name. A key unknown to containerlab's vocabulary is absorbed
	// into the node's kind-specific config instead -- exactly how containerlab itself treats
	// kind-owned keys -- and fails the kind's strict decode when the node is planned.
	for _, want := range []string{"stages", "rejected"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestCompileWrapsKindSpecificConfig(t *testing.T) {
	compiled, nodes, _, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    frr1:
      kind: frr
      image: frr
      daemons: [ospfd]
    ceos1:
      kind: arista_ceos
      image: ceos
      components: []
`)
	frr := compiled.Nodes["frr1"]
	if frr.KindSpecificConfig["daemons"] == nil {
		t.Fatalf("daemons kind config not carried: %#v", frr.KindSpecificConfig)
	}

	ceos := compiled.Nodes["ceos1"]
	if ceos.KindSpecificConfig["components"] == nil {
		t.Fatal("explicit component clearing was not carried")
	}

	frrNode, ok := nodeByName(nodes, "frr1")
	if !ok {
		t.Fatal("the frr node manifest is missing")
	}

	wrapper, ok := frrNode["kind-specific-config"].(map[string]any)
	if !ok {
		t.Fatalf("kind-specific-config wrapper missing from the emitted definition: %#v", frrNode)
	}

	if wrapper["daemons"] == nil {
		t.Fatalf("daemons not wrapped: %#v", wrapper)
	}

	if frrNode["daemons"] != nil {
		t.Fatalf("daemons must not stay at the definition's top level: %#v", frrNode)
	}

	ceosNode, ok := nodeByName(nodes, "ceos1")
	if !ok {
		t.Fatal("the ceos node manifest is missing")
	}

	if ceosNode["components"] == nil {
		t.Fatal("the explicit component clearing did not reach the typed components field")
	}
}

func TestCompileUnwrapsExplicitKindSpecificWrapper(t *testing.T) {
	compiled, _, _, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    frr1:
      kind: frr
      image: frr
      kind-specific-config:
        daemons: [ospfd]
`)
	if compiled.Nodes["frr1"].KindSpecificConfig["daemons"] == nil {
		t.Fatalf("wrapper not unwrapped: %#v", compiled.Nodes["frr1"].KindSpecificConfig)
	}
}

func TestCompileRejectsGenericFieldInsideWrapper(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
topology:
  nodes:
    node1:
      kind: linux
      image: img
      kind-specific-config:
        stages:
          ready:
            wait-for: [other]
`})
	if err == nil {
		t.Fatal("expected the masquerading generic field to be rejected, got none")
	}

	if !strings.Contains(err.Error(), "stages") {
		t.Errorf("error %q does not name the masquerading field", err)
	}
}

func TestCompileRejectsDuplicateKindConfig(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
topology:
  nodes:
    frr1:
      kind: frr
      image: frr
      daemons: [ospfd]
      kind-specific-config:
        daemons: [ospfd]
`})
	if err == nil {
		t.Fatal("expected the duplicate kind config declaration to be rejected, got none")
	}

	if !strings.Contains(err.Error(), "more than once") {
		t.Errorf("error %q does not mention the duplicate: %v", err, err)
	}
}

func TestCompileSanitizesNodeNames(t *testing.T) {
	compiled, nodes, links, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    R1:
      kind: linux
      image: img
      network-mode: container:PE_1
    PE_1:
      kind: linux
      image: img
  links:
    - endpoints: [R1:eth1, PE_1:eth1]
`)
	if compiled.Nodes["r1"] == nil || compiled.Nodes["pe-1"] == nil {
		t.Fatalf("compiled node names = %v, want sanitized names", keys(compiled.Nodes))
	}

	if compiled.SourceNodeName("r1") != "R1" || compiled.SourceNodeName("pe-1") != "PE_1" {
		t.Fatalf("node name sources = %#v, want the definition names", compiled.NodeNameSources)
	}

	if got := compiled.Nodes["r1"].NetworkMode; got != "container:pe-1" {
		t.Fatalf("sanitized network mode = %q, want container:pe-1", got)
	}

	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.GetName())
	}

	if !reflect.DeepEqual(names, []string{"pe-1", "r1"}) {
		t.Fatalf("rendered node names = %v, want sorted sanitized names", names)
	}

	if len(links) != 1 {
		t.Fatalf("rendered links = %d, want 1", len(links))
	}

	spec := links[0].Object["spec"].(map[string]any)
	if spec["endpointA"].(map[string]any)["nodeName"] != "r1" {
		t.Fatalf("link endpoints were not renamed: %#v", spec)
	}
}

func TestCompileRejectsCollidingNodeNames(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
topology:
  nodes:
    r1:
      kind: linux
      image: img
    R1:
      kind: linux
      image: img
`})
	if err == nil {
		t.Fatal("expected the colliding node names to be rejected, got none")
	}

	if !strings.Contains(err.Error(), "both map onto") {
		t.Errorf("error %q does not explain the collision", err)
	}
}

func TestCompileValidatesLinks(t *testing.T) {
	cases := []struct {
		name       string
		definition string
		want       string
	}{
		{
			name: "unknown endpoint node",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
  links:
    - endpoints: [r1:eth1, ghost:eth1]
`,
			want: "nonexistent node",
		},
		{
			name: "special endpoint",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
  links:
    - endpoints: [r1:eth1, mgmt-net:eth1]
`,
			want: "host networking that c9s does not provide",
		},
		{
			name: "unsupported link type",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
    r2: { kind: linux, image: img }
  links:
    - type: vxlan
      endpoints: [r1:eth1, r2:eth1]
`,
			want: "no c9s topology-link equivalent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(&testLogger{}, &Input{Definition: tc.definition})
			if err == nil {
				t.Fatalf("expected a compile error containing %q, got none", tc.want)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestCompileCompilesValidLinks(t *testing.T) {
	compiled, _, links, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
    r2: { kind: linux, image: img }
  links:
    - endpoints: [r1:eth1, r2:eth1]
      mtu: 9000
`)
	if len(compiled.Links) != 1 || len(links) != 1 {
		t.Fatalf("compiled links = %d, rendered = %d, want 1", len(compiled.Links), len(links))
	}

	if compiled.Links[0].MTU != 9000 {
		t.Fatalf("link mtu = %d, want 9000", compiled.Links[0].MTU)
	}

	spec := links[0].Object["spec"].(map[string]any)
	if spec["endpointA"].(map[string]any)["nodeName"] != "r1" ||
		spec["endpointB"].(map[string]any)["nodeName"] != "r2" {
		t.Fatalf("link endpoints = %#v", spec)
	}
}

func TestCompileNormalizesHostPinnedPorts(t *testing.T) {
	logger := &testLogger{}

	compiled, err := Compile(logger, &Input{Definition: `
name: lab
topology:
  nodes:
    r1:
      kind: linux
      image: img
      ports: ["21022:22/tcp"]
`})
	if err != nil {
		t.Fatal(err)
	}

	if got := compiled.Nodes["r1"].Ports[0]; got != "22/tcp" {
		t.Fatalf("normalized port = %q, want 22/tcp", got)
	}

	if len(logger.warnings) == 0 {
		t.Fatal("expected a host pinning warning")
	}
}

func TestCompileConsumesExposePortsLabel(t *testing.T) {
	compiled, _, _, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    r1:
      kind: linux
      image: img
      labels:
        c9s.run/exposePorts: "443"
      ports: ["22/tcp"]
`)
	ports := compiled.Nodes["r1"].Ports
	if len(ports) != 2 || ports[1] != "443/tcp" {
		t.Fatalf("ports = %v, want 22/tcp and 443/tcp", ports)
	}

	if _, present := compiled.Nodes["r1"].Labels["c9s.run/exposePorts"]; present {
		t.Fatal("the expose ports label leaked into the node labels")
	}
}

func TestCompileDropsUnusableLabels(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
topology:
  nodes:
    r1:
      kind: linux
      image: img
      labels:
        "c9s.run/name": owned
        "invalid key!": value
`})
	if err == nil {
		t.Fatal("expected unusable label diagnostics to fail the compile, got none")
	}

	if !strings.Contains(err.Error(), "cannot become a Kubernetes label") {
		t.Errorf("error %q does not mention the label problem", err)
	}
}

func TestCompileValidatesNetworkModes(t *testing.T) {
	cases := []struct {
		name       string
		definition string
		want       string
	}{
		{
			name: "native mode",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img, network-mode: host }
`,
			want: "network-mode",
		},
		{
			name: "unknown primary",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img, network-mode: "container:ghost" }
`,
			want: "nonexistent primary",
		},
		{
			name: "cycle",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img, network-mode: "container:r2" }
    r2: { kind: linux, image: img, network-mode: "container:r1" }
`,
			want: "pod-group cycle",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(&testLogger{}, &Input{Definition: tc.definition})
			if err == nil {
				t.Fatalf("expected a compile error containing %q, got none", tc.want)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestCompileAcceptsContainerNetworkMode(t *testing.T) {
	compiled, _, _, _ := compileForTest(t, `
name: lab
topology:
  nodes:
    primary: { kind: linux, image: img }
    secondary: { kind: linux, image: img, network-mode: "container:primary" }
`)
	if got := compiled.Nodes["secondary"].NetworkMode; got != "container:primary" {
		t.Fatalf("network mode = %q, want container:primary", got)
	}
}

func TestCompileRejectsMoreThanOneManagementNetwork(t *testing.T) {
	_, err := Compile(&testLogger{}, &Input{Definition: `
name: lab
mgmt:
  - ipv4-subnet: 192.0.2.0/24
  - ipv4-subnet: 198.51.100.0/24
topology:
  nodes:
    r1: { kind: linux, image: img }
`})
	if err == nil {
		t.Fatal("expected multiple management networks to be rejected, got none")
	}

	if !strings.Contains(err.Error(), "single management network") {
		t.Errorf("error %q does not explain the limitation", err)
	}
}

func TestCompileCarriesManagementPolicyIntoTheProfile(t *testing.T) {
	_, _, _, profiles := compileForTest(t, `
name: lab
mgmt:
  ipv4-subnet: 192.0.2.0/24
  ipv4-gw: 192.0.2.1
topology:
  nodes:
    r1: { kind: linux, image: img }
`)
	if len(profiles) != 1 {
		t.Fatalf("profiles = %d, want 1", len(profiles))
	}

	mgmt := profiles[0].Object["spec"].(map[string]any)["mgmt"].(map[string]any)
	if mgmt["ipv4-subnet"] != "192.0.2.0/24" || mgmt["ipv4-gw"] != "192.0.2.1" {
		t.Fatalf("management policy = %#v", mgmt)
	}
}

func TestRenderNamesSharedAndDedicatedProfiles(t *testing.T) {
	compiled, nodes, _, profiles := compileForTest(t, `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
    r2: { kind: linux, image: img }
`)
	_ = compiled

	if len(nodes) != 2 || len(profiles) != 1 {
		t.Fatalf(
			"rendered: nodes=%d profiles=%d, want 2 nodes and one shared profile",
			len(nodes),
			len(profiles),
		)
	}

	if profiles[0].GetName() != "lab" {
		t.Fatalf("shared profile name = %q, want lab", profiles[0].GetName())
	}

	for _, n := range nodes {
		spec := n.Object["spec"].(map[string]any)
		profileRef := spec["profileRef"].(map[string]any)
		if profileRef["name"] != "lab" {
			t.Fatalf(
				"node %q references %q, want the shared profile",
				n.GetName(),
				profileRef["name"],
			)
		}
	}
}

func TestRenderPrimitivesCarryContractMetadata(t *testing.T) {
	input := &Input{
		Name:        "lab",
		Namespace:   "ns",
		Definition:  "name: lab\ntopology:\n  nodes:\n    r1: { kind: linux, image: img, group: spine }\n  links:\n    - endpoints: [r1:eth1, r1:eth2]\n",
		Annotations: map[string]string{"note": "hello"},
		Labels:      map[string]string{"tier": "global"},
	}

	compiled, err := Compile(&testLogger{}, input)
	if err != nil {
		t.Fatal(err)
	}

	nodes, links, profiles, err := CompileTopology(input, compiled)
	if err != nil {
		t.Fatal(err)
	}

	node := nodes[0]
	if node.GetAPIVersion() != "c9s.run/v1alpha1" || node.GetKind() != "Node" {
		t.Fatalf("node type meta = %s/%s", node.GetAPIVersion(), node.GetKind())
	}

	if node.GetNamespace() != "ns" || node.GetAnnotations()["note"] != "hello" {
		t.Fatalf("node metadata = %#v", node.Object["metadata"])
	}

	labels := node.Object["metadata"].(map[string]any)["labels"].(map[string]any)
	if labels["c9s.run/app"] != "clabernetes" ||
		labels["c9s.run/topologyOwner"] != "lab" ||
		labels["c9s.run/topologyNode"] != "r1" ||
		labels["c9s.run/topologyGroup"] != "spine" ||
		labels["tier"] != "global" {
		t.Fatalf("node labels = %#v", labels)
	}

	if links[0].GetKind() != "Link" || profiles[0].GetKind() != "NodeProfile" {
		t.Fatalf("primitive kinds = %s/%s", links[0].GetKind(), profiles[0].GetKind())
	}
}

func TestRenderManifestsSurviveKubernetesDeepCopy(t *testing.T) {
	compiled, nodes, links, profiles := compileForTest(t, `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img, ports: ["22/tcp"], env: { A: "b" } }
  links:
    - endpoints: [r1:eth1, r1:eth2]
`)
	_ = compiled

	manifests := make([]unstructured.Unstructured, 0, len(nodes)+len(links)+len(profiles))
	manifests = append(manifests, nodes...)
	manifests = append(manifests, links...)
	manifests = append(manifests, profiles...)

	for i := range manifests {
		// The Kubernetes API server and the dynamic client deep-copy unstructured objects; a
		// manifest carrying a non-JSON-native type (map[string]string, int) panics there.
		if _, err := manifests[i].MarshalJSON(); err != nil {
			t.Fatalf("manifest %q does not marshal: %v", manifests[i].GetName(), err)
		}

		manifests[i].DeepCopy()
	}
}

func keys[V any](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}

	return out
}

func TestCompileUnknownFieldsPointAtTheirSection(t *testing.T) {
	cases := []struct {
		name       string
		definition string
		want       string
	}{
		{
			name: "unknown link field",
			definition: `
name: lab
topology:
  nodes:
    r1: { kind: linux, image: img }
  links:
    - endpoints: [r1:eth1, r1:eth2]
      bogus: true
`,
			want: "topology.links (line",
		},
		{
			name: "unknown topology field",
			definition: `
name: lab
topology:
  bogus: true
  nodes:
    r1: { kind: linux, image: img }
`,
			want: "topology (line",
		},
		{
			name: "unknown lab field",
			definition: `
name: lab
bogus: true
topology:
  nodes:
    r1: { kind: linux, image: img }
`,
			want: "topology (line",
		},
		{
			name: "rejected topology field",
			definition: `
name: lab
topology:
  stages:
    - r1
  nodes:
    r1: { kind: linux, image: img }
`,
			want: `field "stages" is rejected:`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(&testLogger{}, &Input{Definition: tc.definition})
			if err == nil {
				t.Fatalf("expected a compile error containing %q, got none", tc.want)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}

			if strings.Contains(err.Error(), "not found in type") {
				t.Errorf("error %q leaks the raw yaml error", err)
			}
		})
	}
}

func TestCompileMgmtNetWarningPointsAtItsDeclaration(t *testing.T) {
	cases := []struct {
		name       string
		definition string
		wantPath   string
	}{
		{
			name: "node declared",
			definition: `
name: lab
topology:
  nodes:
    r1:
      kind: linux
      image: img
      mgmt-net: mgmt
`,
			wantPath: "topology.nodes.r1.mgmt-net (line",
		},
		{
			name: "inherited from defaults",
			definition: `
name: lab
topology:
  defaults:
    mgmt-net: mgmt
  nodes:
    r1: { kind: linux, image: img }
`,
			wantPath: "topology.defaults.mgmt-net (line",
		},
		{
			name: "inherited from kind",
			definition: `
name: lab
topology:
  kinds:
    linux:
      mgmt-net: mgmt
  nodes:
    r1: { kind: linux, image: img }
`,
			wantPath: "topology.kinds.linux.mgmt-net (line",
		},
		{
			name: "inherited from group",
			definition: `
name: lab
topology:
  groups:
    leafs:
      mgmt-net: mgmt
  nodes:
    r1: { kind: linux, image: img, group: leafs }
`,
			wantPath: "topology.groups.leafs.mgmt-net (line",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := &testLogger{}

			if _, err := Compile(logger, &Input{Definition: tc.definition}); err != nil {
				t.Fatal(err)
			}

			matched := false
			for _, warning := range logger.warnings {
				if strings.Contains(warning, tc.wantPath) {
					matched = true
				}
			}

			if !matched {
				t.Fatalf("warnings %v do not point at %s", logger.warnings, tc.wantPath)
			}
		})
	}
}

// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// absTestdata returns the absolute path of a file of the test data.
func absTestdata(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func TestReadTopologyData(t *testing.T) {
	data, err := readTopologyData(absTestdata(t, "lab-fabric/topology-data.json"))
	if err != nil {
		t.Fatal(err)
	}

	if data.Name != "fabric" || len(data.Nodes) != 4 || len(data.Links) != 5 {
		t.Fatalf("unexpected export: name=%q nodes=%d links=%d", data.Name, len(data.Nodes),
			len(data.Links))
	}

	want := topoDataNode{
		Index:       "0",
		ShortName:   "client1",
		LongName:    "clab-fabric-client1",
		Fqdn:        "client1.fabric.io",
		LabDir:      "/labs/clab-fabric/client1",
		Kind:        "linux",
		Image:       "alpine:3",
		MgmtNet:     "clab",
		MgmtIPv4:    "172.20.20.5",
		MgmtIPv4Len: 24,
		MgmtIPv6:    "3fff:172:20:20::5",
		MgmtIPv6Len: 64,
		Labels: map[string]string{
			"clab-node-kind": "linux", "clab-node-name": "client1", "containerlab": "fabric",
		},
		PortBindings: []topoDataPort{
			{HostIP: "0.0.0.0", HostPort: 8080, Port: 80, Protocol: "tcp"},
		},
	}

	if diff := cmp.Diff(want, data.Nodes["client1"]); diff != "" {
		t.Errorf("unexpected node (-want +got):\n%s", diff)
	}

	if ends := data.Links[4].Endpoints; ends.A == nil || ends.A.Interface != "eth2" ||
		ends.Z != nil {
		t.Errorf("unexpected single ended link: %+v", ends)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "broken.json"), `{"nodes": [1, 2]}`)

	if _, err := readTopologyData(filepath.Join(dir, "broken.json")); err == nil {
		t.Error("an export of another shape was accepted")
	}

	if _, err := readTopologyData(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("a missing export was accepted")
	}
}

func TestReadAnnotations(t *testing.T) {
	ann := readAnnotations(absTestdata(t, "fabric.clab.yml.annotations.json"))
	if ann == nil || len(ann.Nodes) != 4 {
		t.Fatalf("unexpected annotations: %+v", ann)
	}

	leaf2 := ann.node("leaf2")
	if leaf2 == nil || leaf2.Icon != "leaf" || leaf2.Level != "2" || leaf2.GroupID != "racks" ||
		leaf2.Position == nil || leaf2.Position.X != 400 || leaf2.Position.Y != 260 {
		t.Errorf("unexpected annotation of leaf2: %+v", leaf2)
	}

	if spine := ann.node("spine1"); spine == nil || spine.Icon != "" || spine.Level != "" {
		t.Errorf("unexpected annotation of spine1: %+v", spine)
	}

	if ann.node("client1") != nil {
		t.Error("an annotation was found for a node that has none")
	}

	// The file is optional and may be anything.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "broken.json"), `{"nodeAnnotations": "none"}`)
	writeFile(t, filepath.Join(dir, "odd.json"),
		`{"nodeAnnotations": [{"id": "n1", "level": true, "position": null}]}`)

	if readAnnotations(filepath.Join(dir, "broken.json")) != nil {
		t.Error("broken annotations were accepted")
	}

	if readAnnotations(filepath.Join(dir, "missing.json")) != nil {
		t.Error("missing annotations were accepted")
	}

	odd := readAnnotations(filepath.Join(dir, "odd.json"))
	if n := odd.node("n1"); n == nil || n.Level != "" || n.Position != nil {
		t.Errorf("unexpected annotation with values of other types: %+v", n)
	}

	var none *annotations
	if none.node("n1") != nil {
		t.Error("annotations that do not exist returned a node")
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]int{
		"": 0, "1": 1, "L1": 1, "l3": 3, "tier 12": 12, "2a": 2, "spine": 0, "-4": 4,
	} {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %d, want %d", in, got, want)
		}
	}
}

// wantFabricNodes are the nodes of the fabric topology of the test data with what its
// drawing file and labels say about them. The kinds differ between the deployed lab and
// the topology file.
func wantFabricNodes(leafKind string) []clabinternaltuibackend.TopoNode {
	return []clabinternaltuibackend.TopoNode{
		{Name: "client1", Kind: "linux"},
		// The drawing file wins over the labels and the group of the node.
		{
			Name: "leaf1", Kind: leafKind, Group: "group-leaves", Role: "pe", Level: 3,
			X: 200.5, Y: 260, HasPos: true,
		},
		{
			Name: "leaf2", Kind: leafKind, Group: "racks", Role: "leaf", Level: 2,
			X: 400, Y: 260, HasPos: true,
		},
		// Without icon and level in the drawing file the labels decide.
		{Name: "spine1", Kind: leafKind, Role: "spine", Level: 1, X: 300, Y: 100, HasPos: true},
	}
}

func TestTopologyFromData(t *testing.T) {
	data, err := readTopologyData(absTestdata(t, "lab-fabric/topology-data.json"))
	if err != nil {
		t.Fatal(err)
	}

	ann := readAnnotations(absTestdata(t, "fabric.clab.yml.annotations.json"))
	got := topologyFromData("fabric", data, ann)

	want := clabinternaltuibackend.Topology{
		Lab:   "fabric",
		Nodes: wantFabricNodes("nokia_srlinux"),
		Links: []clabinternaltuibackend.Link{
			{
				A: clabinternaltuibackend.Endpoint{
					Node: "leaf1", Interface: "e1-1", MAC: "aa:c1:ab:76:42:b0",
				},
				Z: &clabinternaltuibackend.Endpoint{
					Node: "spine1", Interface: "e1-1", MAC: "aa:c1:ab:5b:85:30",
				},
			},
			{
				A: clabinternaltuibackend.Endpoint{
					Node: "leaf2", Interface: "e1-1", MAC: "aa:c1:ab:00:00:01",
				},
				Z: &clabinternaltuibackend.Endpoint{
					Node: "spine1", Interface: "e1-2", MAC: "aa:c1:ab:00:00:02",
				},
			},
			{
				A: clabinternaltuibackend.Endpoint{
					Node: "client1", Interface: "eth1", MAC: "aa:c1:ab:00:00:03",
				},
				Z: &clabinternaltuibackend.Endpoint{
					Node: "leaf1", Interface: "ethernet-1/2", MAC: "aa:c1:ab:00:00:04",
				},
			},
			{
				A: clabinternaltuibackend.Endpoint{
					Node: "leaf1", Interface: "e1-3", MAC: "aa:c1:ab:00:00:05",
				},
				// The host is no node of the lab.
				Z: &clabinternaltuibackend.Endpoint{
					Node: "host", Interface: "leaf1-e1-3", MAC: "aa:c1:ab:00:00:06",
					External: true,
				},
			},
			{
				A: clabinternaltuibackend.Endpoint{
					Node: "client1", Interface: "eth2", MAC: "aa:c1:ab:00:00:07",
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected topology (-want +got):\n%s", diff)
	}

	// Without a drawing file the labels and the group of the node are all there is.
	plain := topologyFromData("fabric", data, nil)
	wantLeaf1 := clabinternaltuibackend.TopoNode{
		Name: "leaf1", Kind: "nokia_srlinux", Group: "fabric", Role: "leaf", Level: 2,
	}

	if diff := cmp.Diff(wantLeaf1, plain.Nodes[1]); diff != "" {
		t.Errorf("unexpected node without a drawing (-want +got):\n%s", diff)
	}

	if plain.Nodes[2].Group != "rack2" {
		t.Errorf("group of leaf2 = %q, want the one of the node", plain.Nodes[2].Group)
	}
}

// parseWithoutRuntime parses a topology file the way the backend does, less the container
// runtime, which a parse does not need.
func parseWithoutRuntime(file string) (*clabcore.CLab, error) {
	return clabcore.NewContainerLab(
		clabcore.WithSkippedBindsPathsCheck(),
		clabcore.WithTopoPath(file, nil),
	)
}

func TestTopologyFromTopologyFile(t *testing.T) {
	file := absTestdata(t, "fabric.clab.yml")

	c, err := parseWithoutRuntime(file)
	if err != nil {
		t.Fatal(err)
	}

	got := topologyFromCLab("fabric", c, readAnnotations(file+annotationsSuffix))
	want := clabinternaltuibackend.Topology{
		Lab:   "fabric",
		Nodes: wantFabricNodes("linux"),
		Links: []clabinternaltuibackend.Link{
			{
				A: clabinternaltuibackend.Endpoint{Node: "leaf1", Interface: "eth1"},
				Z: &clabinternaltuibackend.Endpoint{Node: "spine1", Interface: "eth1"},
			},
			{
				A: clabinternaltuibackend.Endpoint{Node: "leaf2", Interface: "eth1"},
				Z: &clabinternaltuibackend.Endpoint{Node: "spine1", Interface: "eth2"},
			},
			{
				A: clabinternaltuibackend.Endpoint{Node: "client1", Interface: "eth1"},
				Z: &clabinternaltuibackend.Endpoint{Node: "leaf1", Interface: "eth2"},
			},
			{
				A: clabinternaltuibackend.Endpoint{Node: "leaf1", Interface: "eth3"},
				Z: &clabinternaltuibackend.Endpoint{
					Node: "host", Interface: "leaf1-eth3", External: true,
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected topology (-want +got):\n%s", diff)
	}
}

// fabricContainers returns the containers of the deployed fabric lab of the test data.
func fabricContainers(t *testing.T) []clabruntime.GenericContainer {
	t.Helper()

	topo := absTestdata(t, "fabric.clab.yml")
	labDir := absTestdata(t, "lab-fabric")

	containers := make([]clabruntime.GenericContainer, 0, 4)

	for _, name := range []string{"leaf1", "leaf2", "spine1", "client1"} {
		ctr := nodeContainer("fabric", name, topo)
		ctr.Labels[clabconstants.NodeLabDir] = filepath.Join(labDir, name)
		containers = append(containers, ctr)
	}

	return containers
}

func TestTopologyOfDeployedLab(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	topo := b.Topology("fabric")
	if topo.Warning != "" || len(topo.Nodes) != 4 || len(topo.Links) != 5 {
		t.Fatalf("unexpected topology: %+v", topo)
	}

	if diff := cmp.Diff(wantFabricNodes("nokia_srlinux"), topo.Nodes); diff != "" {
		t.Errorf("unexpected nodes (-want +got):\n%s", diff)
	}

	// What is handed out is a copy.
	topo.Nodes[0].Name = "changed"
	topo.Links[0].Z.Node = "changed"

	if again := b.Topology("fabric"); again.Nodes[0].Name != "client1" ||
		again.Links[0].Z.Node != "spine1" {
		t.Errorf("a change of a returned topology reached the backend: %+v", again)
	}

	if unknown := b.Topology("nope"); unknown.Lab != "nope" || unknown.Warning == "" {
		t.Errorf("unexpected topology of an unknown lab: %+v", unknown)
	}
}

func TestTopologyIsReadAgainWhenItsFilesChange(t *testing.T) {
	dir := t.TempDir()
	topoFile := filepath.Join(dir, "lab1.clab.yml")
	export := filepath.Join(dir, "clab-lab1", topologyDataFile)

	writeFile(t, topoFile, "name: lab1\n")
	writeFile(t, export, `{"name": "lab1", "nodes": {"n1": {"kind": "linux"}}, "links": []}`)

	ctr := nodeContainer("lab1", "n1", topoFile)
	ctr.Labels[clabconstants.NodeLabDir] = filepath.Join(dir, "clab-lab1", "n1")

	b := newTestBackend(t)
	b.setLabs(ctr)

	if topo := b.Topology("lab1"); len(topo.Nodes) != 1 || topo.Nodes[0].HasPos {
		t.Fatalf("unexpected topology: %+v", topo)
	}

	// The lab is deployed anew with a second node.
	writeFile(t, export, `{"name": "lab1", "nodes": {"n1": {"kind": "linux"}, `+
		`"n2": {"kind": "linux"}}, "links": [{"endpoints": {"a": {"node": "n1", `+
		`"interface": "eth1"}, "z": {"node": "n2", "interface": "eth1"}}}]}`)

	if topo := b.Topology("lab1"); len(topo.Nodes) != 2 || len(topo.Links) != 1 {
		t.Errorf("the changed export was not read: %+v", topo)
	}

	// A drawing is saved for the topology.
	writeFile(t, topoFile+annotationsSuffix,
		`{"nodeAnnotations": [{"id": "n2", "position": {"x": 7, "y": 9}}]}`)

	if topo := b.Topology("lab1"); !topo.Nodes[1].HasPos || topo.Nodes[1].X != 7 {
		t.Errorf("the new drawing was not read: %+v", topo)
	}

	// Once the export is gone, the nodes are still known from the inventory.
	if err := os.Remove(export); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(topoFile); err != nil {
		t.Fatal(err)
	}

	b.setLabs(ctr)

	topo := b.Topology("lab1")
	if len(topo.Nodes) != 1 || topo.Nodes[0].Name != "n1" || len(topo.Links) != 0 ||
		!strings.Contains(topo.Warning, "links are unknown") {
		t.Errorf("unexpected topology without export and topology file: %+v", topo)
	}
}

func TestTopologyOfTopologyFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "fabric.clab.yml")

	source, err := os.ReadFile(absTestdata(t, "fabric.clab.yml"))
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, file, string(source))

	var parses atomic.Int32

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.loadTopo = func(file string) (*clabcore.CLab, error) {
			parses.Add(1)

			return parseWithoutRuntime(file)
		}
	})
	b.files.replace([]topoFile{readTopoFile(file)})

	// The file is parsed in the background, the first answer is a placeholder.
	first := b.Topology("fabric")
	if first.Lab != "fabric" || first.Warning == "" || len(first.Nodes) != 0 {
		t.Errorf("unexpected first answer: %+v", first)
	}

	eventually(t, "the topology to be parsed", func() bool {
		return b.drainTopics().Has(clabinternaltuibackend.TopicLinks)
	})

	parsed := b.Topology("fabric")
	if parsed.Warning != "" || len(parsed.Nodes) != 4 || len(parsed.Links) != 4 {
		t.Fatalf("unexpected topology: %+v", parsed)
	}

	// Asking again does not parse again.
	b.Topology("fabric")

	if got := parses.Load(); got != 1 {
		t.Errorf("the file was parsed %d times, want once", got)
	}

	// A changed file is parsed anew. Until that is done the previous result is served.
	changed := strings.Replace(string(source), "    client1:\n", "    client1:\n    client2:\n", 1)
	writeFile(t, file, changed)

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}

	if stale := b.Topology("fabric"); len(stale.Nodes) != 4 {
		t.Errorf("unexpected answer while parsing anew: %+v", stale)
	}

	eventually(t, "the changed topology to be parsed", func() bool {
		return len(b.Topology("fabric").Nodes) == 5
	})
}

func TestTopologyOfBrokenTopologyFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "broken.clab.yml")
	writeFile(t, file, "name: broken\n")

	var parses atomic.Int32

	b := newTestBackend(t, func(_ *Options, d *deps) {
		d.loadTopo = func(string) (*clabcore.CLab, error) {
			parses.Add(1)

			return nil, errors.New("no kind\nset for node \x1b[1mn1\x1b[0m")
		}
	})
	b.files.replace([]topoFile{readTopoFile(file)})

	b.Topology("broken")

	eventually(t, "the parse to fail", func() bool {
		return strings.Contains(b.Topology("broken").Warning, "failed to read")
	})

	want := "failed to read the topology: no kind set for node n1"
	if got := b.Topology("broken").Warning; got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}

	// The failure is remembered for as long as the file stays the same.
	if got := parses.Load(); got != 1 {
		t.Errorf("the file was parsed %d times, want once", got)
	}
}

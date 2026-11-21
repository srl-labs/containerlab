// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// appDrawing is a drawing file as the containerlab application writes it.
const appDrawing = `{
  "freeTextAnnotations": [],
  "groupStyleAnnotations": [
    {
      "id": "dc:1",
      "color": "#00c9ff"
    }
  ],
  "nodeAnnotations": [
    {
      "id": "spine1",
      "interfacePattern": "e1-{n}",
      "position": {
        "x": 380,
        "y": 320
      },
      "icon": "spine"
    },
    {
      "id": "leaf1",
      "groupId": "dc:1"
    }
  ],
  "edgeAnnotations": [],
  "viewerSettings": {
    "nodeStyle": "boxed"
  }
}`

func TestPlaceNodesChangesNothingButThePositions(t *testing.T) {
	topo := filepath.Join(t.TempDir(), "lab.clab.yml")
	path := topo + annotationsSuffix

	if err := os.WriteFile(path, []byte(appDrawing), 0o640); err != nil {
		t.Fatal(err)
	}

	err := placeNodes(path, topo, map[string]clabinternaltuibackend.Position{
		"spine1": {X: 401.256, Y: 320},
		"leaf1":  {X: -20, Y: 99.5},
		"leaf2":  {X: 7, Y: 8},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := strings.NewReplacer(
		`"x": 380,`, `"x": 401.26,`,
		`      "groupId": "dc:1"
    }`, `      "groupId": "dc:1",
      "position": {
        "x": -20,
        "y": 99.5
      }
    },
    {
      "id": "leaf2",
      "position": {
        "x": 7,
        "y": 8
      }
    }`,
	).Replace(appDrawing)

	if string(got) != want {
		t.Errorf("the drawing file is\n%s\nwant\n%s", got, want)
	}

	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("the mode of the file is %v, want it kept", info.Mode().Perm())
	}

	// What is read back is what was saved.
	ann := readAnnotations(path)
	if pos := ann.node("leaf2").Position; pos == nil || pos.X != 7 || pos.Y != 8 {
		t.Errorf("leaf2 is read back at %v", pos)
	}

	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".*")); len(leftovers) > 0 {
		t.Errorf("temporary files are left behind: %v", leftovers)
	}
}

func TestPlaceNodesStartsADrawing(t *testing.T) {
	topo := filepath.Join(t.TempDir(), "lab.clab.yml")
	path := topo + annotationsSuffix

	err := placeNodes(path, topo, map[string]clabinternaltuibackend.Position{
		"b": {X: 1, Y: 2}, "a": {X: 3, Y: 4},
	})
	if err != nil {
		t.Fatal(err)
	}

	ann := readAnnotations(path)
	if ann == nil || len(ann.Nodes) != 2 || ann.Nodes[0].ID != "a" || ann.Nodes[1].ID != "b" {
		t.Fatalf("the new drawing holds %+v", ann)
	}

	if pos := ann.node("a").Position; pos == nil || pos.X != 3 || pos.Y != 4 {
		t.Errorf("a is at %v", pos)
	}
}

func TestPlaceNodesLeavesWhatItCannotReadAlone(t *testing.T) {
	for name, content := range map[string]string{
		"not json":      "nodeAnnotations: []",
		"not an object": `["nodeAnnotations"]`,
		"not a list":    `{"nodeAnnotations": {"spine1": {}}}`,
		"two values":    `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			topo := filepath.Join(t.TempDir(), "lab.clab.yml")
			path := topo + annotationsSuffix

			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			err := placeNodes(path, topo, map[string]clabinternaltuibackend.Position{"a": {X: 1}})
			if err == nil {
				t.Error("no error for a file that holds no drawing")
			}

			if got, _ := os.ReadFile(path); string(got) != content {
				t.Errorf("the file was changed to %q", got)
			}
		})
	}
}

func TestPlaceNodesKeepsTheEndOfTheFile(t *testing.T) {
	topo := filepath.Join(t.TempDir(), "lab.clab.yml")
	path := topo + annotationsSuffix

	if err := os.WriteFile(path, []byte(appDrawing+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The same position again: the file is not even written.
	before, _ := os.Stat(path)

	if err := placeNodes(path, topo, map[string]clabinternaltuibackend.Position{
		"spine1": {X: 380, Y: 320},
	}); err != nil {
		t.Fatal(err)
	}

	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the file was written although nothing changed")
	}

	if err := placeNodes(path, topo, map[string]clabinternaltuibackend.Position{
		"spine1": {X: 381, Y: 320},
	}); err != nil {
		t.Fatal(err)
	}

	if got, _ := os.ReadFile(path); !strings.HasSuffix(string(got), "}\n") ||
		strings.HasSuffix(string(got), "\n\n") {
		t.Errorf("the file ends with %q", got[len(got)-3:])
	}
}

func TestBackendPlaceNodes(t *testing.T) {
	topo := filepath.Join(t.TempDir(), "fabric.clab.yml")
	labDir := absTestdata(t, "lab-fabric")

	containers := make([]clabruntime.GenericContainer, 0, 4)

	for _, name := range []string{"leaf1", "leaf2", "spine1", "client1"} {
		ctr := nodeContainer("fabric", name, topo)
		ctr.Labels[clabconstants.NodeLabDir] = filepath.Join(labDir, name)
		containers = append(containers, ctr)
	}

	if err := os.WriteFile(topo, []byte("name: fabric\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	b := newTestBackend(t)
	b.setLabs(containers...)

	for _, n := range b.Topology("fabric").Nodes {
		if n.HasPos {
			t.Fatalf("%s has a position before any was saved", n.Name)
		}
	}

	b.drainTopics()

	err := b.PlaceNodes("fabric", map[string]clabinternaltuibackend.Position{
		"leaf1": {X: 120.5, Y: 240}, "spine1": {X: 300, Y: 80},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !b.drainTopics().Has(clabinternaltuibackend.TopicLinks) {
		t.Error("the UI is not told that the topology changed")
	}

	placed := map[string]clabinternaltuibackend.TopoNode{}
	for _, n := range b.Topology("fabric").Nodes {
		placed[n.Name] = n
	}

	if n := placed["leaf1"]; !n.HasPos || n.X != 120.5 || n.Y != 240 {
		t.Errorf("leaf1 is reported at %v,%v (%v)", n.X, n.Y, n.HasPos)
	}

	if placed["leaf2"].HasPos {
		t.Error("leaf2 has a position although none was saved for it")
	}

	if err := b.PlaceNodes(
		"nope",
		map[string]clabinternaltuibackend.Position{"a": {}},
	); err == nil {
		t.Error("no error for a lab that is not known")
	}

	readOnly := newTestBackend(t, func(o *Options, _ *deps) { o.ReadOnly = true })
	readOnly.setLabs(containers...)

	err = readOnly.PlaceNodes("fabric", map[string]clabinternaltuibackend.Position{"leaf1": {}})
	if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) {
		t.Errorf("a backend that changes nothing saved a drawing: %v", err)
	}
}

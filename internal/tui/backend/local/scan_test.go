// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var unlimitedScan = scanLimits{dirs: scanMaxDirs, files: scanMaxFiles}

// relativeTo returns the paths relative to root, sorted.
func relativeTo(t *testing.T, root string, paths []string) []string {
	t.Helper()

	out := make([]string, 0, len(paths))

	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, rel)
	}

	slices.Sort(out)

	return out
}

func TestFindTopologyFiles(t *testing.T) {
	root := t.TempDir()

	for _, file := range []string{
		"top.clab.yml",
		"other.clab.yaml",
		"notes.yml",
		"clab.yml",
		"backup.clab.yml.bak",
		"a/one.clab.yml",
		"a/b/two.clab.yml",
		"a/b/c/three.clab.yml",
		"a/b/c/d/four.clab.yml",
		// Hidden directories and directories of third party code are not entered.
		".git/hidden.clab.yml",
		".hidden/deeper/hidden.clab.yml",
		"node_modules/pkg/dep.clab.yml",
		"__pycache__/cached.clab.yml",
		"vendor/lib/vendored.clab.yml",
		// Neither are lab directories, whatever marks them.
		"clab-lab1/topology-data.json",
		"clab-lab1/copy.clab.yml",
		"clab-lab1/node1/nested.clab.yml",
		"clab-lab2/.state.clab.yaml",
		"clab-lab2/copy.clab.yml",
		// Directories that only look like the ones skipped are searched.
		"vendors/real.clab.yml",
		"my.vendor/real.clab.yml",
	} {
		writeFile(t, filepath.Join(root, file), "name: x\n")
	}

	// A directory is not a topology file, whatever its name.
	if err := os.Mkdir(filepath.Join(root, "dir.clab.yml"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Links to directories are not followed, links to files are.
	elsewhere := t.TempDir()
	writeFile(t, filepath.Join(elsewhere, "linked.clab.yml"), "name: linked\n")

	if err := os.Symlink(elsewhere, filepath.Join(root, "link-to-dir")); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(elsewhere, "linked.clab.yml"),
		filepath.Join(root, "link.clab.yml")); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(root, "missing"),
		filepath.Join(root, "dangling.clab.yml")); err != nil {
		t.Fatal(err)
	}

	tests := map[int][]string{
		0: {"link.clab.yml", "other.clab.yaml", "top.clab.yml"},
		1: {
			"a/one.clab.yml", "link.clab.yml", "my.vendor/real.clab.yml", "other.clab.yaml",
			"top.clab.yml", "vendors/real.clab.yml",
		},
		3: {
			"a/b/c/three.clab.yml", "a/b/two.clab.yml", "a/one.clab.yml", "link.clab.yml",
			"my.vendor/real.clab.yml", "other.clab.yaml", "top.clab.yml",
			"vendors/real.clab.yml",
		},
	}

	for depth, want := range tests {
		t.Run(fmt.Sprintf("depth %d", depth), func(t *testing.T) {
			files, truncated := findTopologyFiles(context.Background(),
				[]scanRoot{{dir: root, depth: depth}}, unlimitedScan)
			if truncated {
				t.Error("the scan reports that it hit a limit")
			}

			if diff := cmp.Diff(want, relativeTo(t, root, files)); diff != "" {
				t.Errorf("unexpected files (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFindTopologyFilesStopsAtItsLimits(t *testing.T) {
	root := t.TempDir()

	for i := range 10 {
		writeFile(t, filepath.Join(root, fmt.Sprintf("d%d/lab.clab.yml", i)), "name: x\n")
	}

	roots := []scanRoot{{dir: root, depth: 2}}

	// The root and three of its directories fit into a limit of four directories.
	files, truncated := findTopologyFiles(
		context.Background(),
		roots,
		scanLimits{dirs: 4, files: 100},
	)
	if !truncated || len(files) != 3 {
		t.Errorf("directory limit: %d files, truncated=%v, want 3 files and a truncated scan",
			len(files), truncated)
	}

	files, truncated = findTopologyFiles(
		context.Background(),
		roots,
		scanLimits{dirs: 100, files: 5},
	)
	if !truncated || len(files) != 5 {
		t.Errorf("file limit: %d files, truncated=%v, want 5 files and a truncated scan",
			len(files), truncated)
	}

	files, truncated = findTopologyFiles(
		context.Background(),
		roots,
		scanLimits{dirs: 11, files: 10},
	)
	if truncated || len(files) != 10 {
		t.Errorf("exact fit: %d files, truncated=%v, want 10 files and a complete scan",
			len(files), truncated)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if files, _ := findTopologyFiles(ctx, roots, unlimitedScan); len(files) != 0 {
		t.Errorf("a canceled scan found %d files", len(files))
	}
}

func TestScanRoots(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	labs := filepath.Join(home, "labs")

	for _, dir := range []string{labs, filepath.Join(home, ".clab")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	alias := filepath.Join(base, "alias")
	if err := os.Symlink(labs, alias); err != nil {
		t.Fatal(err)
	}

	got := scanRoots([]string{
		labs,
		home,
		// The same directory again, spelled differently and through a link.
		labs + string(filepath.Separator) + ".",
		alias,
		filepath.Join(base, "missing"),
		"",
		string(filepath.Separator),
		filepath.Join(home, ".clab"),
	}, home, 3)

	want := []scanRoot{
		{dir: labs, depth: 3},
		// The home and the root directory are only looked at one level deep.
		{dir: home, depth: 1},
		{dir: string(filepath.Separator), depth: 1},
		{dir: filepath.Join(home, ".clab"), depth: 3},
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(scanRoot{})); diff != "" {
		t.Errorf("unexpected roots (-want +got):\n%s", diff)
	}

	// A configured depth below the shallow one is kept.
	if got := scanRoots([]string{home}, home, 0); len(got) != 1 || got[0].depth != 0 {
		t.Errorf("unexpected roots for depth 0: %+v", got)
	}
}

func TestReadTopoFile(t *testing.T) {
	dir := t.TempDir()

	tests := map[string]struct {
		content   string
		wantName  string
		wantNodes int
	}{
		"regular topology": {
			content: "name: fabric\ntopology:\n  defaults:\n    kind: nokia_srlinux\n" +
				"  nodes:\n    leaf1:\n    leaf2:\n      labels: {role: leaf}\n    spine1: {}\n" +
				"  links:\n    - endpoints: [\"leaf1:e1-1\", \"spine1:e1-1\"]\n",
			wantName:  "fabric",
			wantNodes: 3,
		},
		"no nodes": {
			content:  "name: empty\ntopology:\n  links: []\n",
			wantName: "empty",
		},
		"no name falls back to the file name": {
			content:   "topology:\n  nodes:\n    n1:\n",
			wantName:  "no name falls back to the file name",
			wantNodes: 1,
		},
		"a template falls back to the file name": {
			content: "name: {{ .name }}\ntopology:\n  nodes:\n" +
				"{{- range .nodes }}\n    x:\n{{- end }}\n",
			wantName: "a template falls back to the file name",
		},
		"a name with control characters is cleaned": {
			content:   "name: \"evil\\e[31m\"\ntopology:\n  nodes: {a: {}}\n",
			wantName:  "evil",
			wantNodes: 1,
		},
		"not YAML at all": {
			content:  "\x00\x01\x02 binary",
			wantName: "not YAML at all",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".clab.yml")
			writeFile(t, path, tt.content)

			got := readTopoFile(path)
			if got.name != tt.wantName || got.nodes != tt.wantNodes || got.path != path ||
				got.real == "" {
				t.Errorf("got %+v, want name %q and %d nodes", got, tt.wantName, tt.wantNodes)
			}
		})
	}

	if got := readTopoFile(filepath.Join(dir, "gone.clab.yaml")); got.name != "gone" ||
		got.real != "" {
		t.Errorf("unexpected result for a missing file: %+v", got)
	}
}

func TestTopologyFileMatching(t *testing.T) {
	dir := t.TempDir()
	topo := filepath.Join(dir, "lab1.clab.yml")
	writeFile(t, topo, "name: lab1\n")

	linkDir := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, linkDir); err != nil {
		t.Fatal(err)
	}

	copied := filepath.Join(t.TempDir(), "copy.clab.yml")
	writeFile(t, copied, "name: lab1\n")

	deployed := buildLabs(
		[]clabruntime.GenericContainer{nodeContainer("lab1", "n1", topo)},
		&inventoryEnv{exists: clabutils.FileOrDirExists, sshxLink: func(string) sshxLink { return sshxLink{} }},
	)[0]
	moved := nodeContainer("lab1", "n1", filepath.Join(dir, "moved.clab.yml"))
	orphaned := buildLabs(
		[]clabruntime.GenericContainer{moved},
		&inventoryEnv{exists: clabutils.FileOrDirExists, sshxLink: func(string) sshxLink { return sshxLink{} }},
	)[0]

	if deployed.lab.TopoMissing || !orphaned.lab.TopoMissing {
		t.Fatalf("unexpected labs: %+v, %+v", deployed.lab, orphaned.lab)
	}

	tests := map[string]struct {
		file topoFile
		lab  *labRecord
		want bool
	}{
		"the file of the lab": {readTopoFile(topo), deployed, true},
		"the file of the lab reached through a link": {
			readTopoFile(filepath.Join(linkDir, "lab1.clab.yml")), deployed, true,
		},
		// Two files may define a lab of the same name. Only one of them is deployed.
		"another file defining a lab of the same name": {readTopoFile(copied), deployed, false},
		"a lab whose file is gone is matched by name":  {readTopoFile(copied), orphaned, true},
		"another lab": {
			topoFile{path: copied, real: copied, name: "lab2"}, orphaned, false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.file.belongsTo(tt.lab); got != tt.want {
				t.Errorf("belongsTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabsListsTopologyFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "idle10.clab.yml"), "name: idle10\n")
	writeFile(t, filepath.Join(dir, "sub", "idle2.clab.yaml"),
		"name: idle2\ntopology:\n  nodes: {a: {}}\n")
	// A lab directory holds a copy, which is not listed.
	writeFile(t, filepath.Join(dir, "clab-idle2", topologyDataFile), "{}")
	writeFile(t, filepath.Join(dir, "clab-idle2", "idle2.clab.yml"), "name: idle2\n")

	// The lab is deployed from elsewhere. Its file is listed, the files next to it are not.
	deployedDir := t.TempDir()
	deployedTopo := filepath.Join(deployedDir, "running.clab.yml")
	writeFile(t, deployedTopo, "name: running\ntopology:\n  nodes: {n1: {}, n2: {}, n3: {}}\n")
	writeFile(t, filepath.Join(deployedDir, "neighbor.clab.yml"), "name: neighbor\n")

	other := t.TempDir()
	writeFile(t, filepath.Join(other, "extra.clab.yml"), "name: extra\n")

	// The home directory is searched too, the one of the user running the tests must not
	// matter.
	t.Setenv("HOME", t.TempDir())

	b := newTestBackend(t, func(o *Options, _ *deps) { o.LabsDirs = []string{other} })
	b.setLabs(
		nodeContainer("running", "n1", deployedTopo),
		nodeContainer("running", "n2", deployedTopo),
	)

	t.Chdir(dir)

	files := b.scanTopologies(context.Background())
	b.files.replace(files)

	type entry struct {
		Name     string
		File     string
		Deployed bool
		Defined  int
		Nodes    int
	}

	var got []entry

	for _, lab := range b.Labs() {
		if strings.HasPrefix(lab.TopoFile, dir) || strings.HasPrefix(lab.TopoFile, other) ||
			strings.HasPrefix(lab.TopoFile, deployedDir) {
			got = append(
				got,
				entry{lab.Name, lab.TopoFile, lab.Deployed, lab.DefinedNodes, len(lab.Nodes)},
			)
		}
	}

	want := []entry{
		{"running", deployedTopo, true, 3, 2},
		{"extra", filepath.Join(other, "extra.clab.yml"), false, 0, 0},
		{"idle2", filepath.Join(dir, "sub", "idle2.clab.yaml"), false, 1, 0},
		{"idle10", filepath.Join(dir, "sub", "idle10.clab.yml"), false, 0, 0},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected labs (-want +got):\n%s", diff)
	}

	// A job marks its lab busy, be it deployed or not.
	if _, err := b.Deploy(clabinternaltuibackend.DeployRequest{
		Lab: "idle2", TopoFile: filepath.Join(dir, "sub", "idle2.clab.yaml"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, lab := range b.Labs() {
		if want := lab.Name == "idle2"; lab.Busy != want {
			t.Errorf("lab %s: Busy = %v, want %v", lab.Name, lab.Busy, want)
		}
	}
}

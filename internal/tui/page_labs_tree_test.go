// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

// treeOutline writes a folder the way the list shows it: one line per lab and folder,
// indented by depth.
func treeOutline(labs []clabinternaltuibackend.Lab, f *labFolder, depth int) []string {
	var lines []string

	pad := strings.Repeat("  ", depth)

	for _, i := range f.labs {
		lines = append(lines, pad+labs[i].Name)
	}

	for _, sub := range f.folders {
		lines = append(lines, fmt.Sprintf("%s%s/ (%d)", pad, sub.name, sub.count))
		lines = append(lines, treeOutline(labs, sub, depth+1)...)
	}

	return lines
}

func TestLabTree(t *testing.T) {
	file := func(name, path string) clabinternaltuibackend.Lab {
		return clabinternaltuibackend.Lab{Name: name, TopoFile: path}
	}

	labs := []clabinternaltuibackend.Lab{
		{Name: "running", TopoFile: "/home/u/labs/running.clab.yml", Deployed: true},
		file("srl", "/home/u/labs/srl.clab.yml"),
		file("frr", "/home/u/labs/training/frr.clab.yml"),
		file("ebgp", "/home/u/labs/training/bgp/ebgp.clab.yml"),
		file("deep", "/home/u/labs/a/b/c/deep.clab.yml"),
		file("st", "/home/u/.clab/srl-labs/srl-telemetry-lab/st.clab.yml"),
		file("extra", "/data/labs/extra.clab.yml"),
		file("moved", "/srv/elsewhere/moved.clab.yml"),
		{Name: "nofile"},
	}
	dirs := []string{"/home/u/labs", "/data/labs", "/home/u/.clab"}

	got := treeOutline(labs, labTree(labs, dirs, "/home/u"), 0)
	want := []string{
		// The labs of the start directory are at the top, with its folders.
		"nofile",
		"srl",
		"a/b/c/ (1)", // a chain of folders is one
		"  deep",
		"training/ (2)",
		"  frr",
		"  bgp/ (1)",
		"    ebgp",
		// The other directories searched, in their order, then the rest.
		"/data/labs/ (1)",
		"  extra",
		"~/.clab/srl-labs/srl-telemetry-lab/ (1)",
		"  st",
		"/srv/elsewhere/ (1)",
		"  moved",
	}

	if !slices.Equal(got, want) {
		t.Errorf("tree:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLabTreeFromRoot: started in the root directory, the home is searched in its own
// right and keeps its own folder.
func TestLabTreeFromRoot(t *testing.T) {
	labs := []clabinternaltuibackend.Lab{
		{Name: "top", TopoFile: "/top.clab.yml"},
		{Name: "opt", TopoFile: "/opt/x/opt.clab.yml"},
		{Name: "st", TopoFile: "/home/u/.clab/st/st.clab.yml"},
	}

	got := treeOutline(labs, labTree(labs, []string{"/", "/home/u/.clab"}, "/home/u"), 0)
	want := []string{"top", "opt/x/ (1)", "  opt", "~/.clab/st/ (1)", "  st"}

	if !slices.Equal(got, want) {
		t.Errorf("tree:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLabFolders walks the folders of the labs that are not deployed with the keys a
// user would press: enter and the arrows open and close them, e opens all of them, and
// the filter finds a lab in a closed folder.
func TestLabFolders(t *testing.T) {
	m, _ := journey(t)
	page := m.top().(*labsPage)

	if !page.tbl.SetCursorKey(folderKey("/home/demo/labs/training")) {
		t.Fatal("there is no folder for the training directory")
	}

	press(m, "enter")

	if _, open := page.tbl.CursorFolder(); !open {
		t.Fatal("enter did not open the folder")
	}

	press(m, "right")

	if lab := page.current(m); lab == nil || lab.Name != "frr01" {
		t.Fatalf("right into the folder led to %+v", lab)
	}

	press(m, "left")

	if f := page.currentFolder(); f == nil || f.name != "training" {
		t.Fatalf("left out of the folder led to %+v", f)
	}

	press(m, "left")

	if _, open := page.tbl.CursorFolder(); open {
		t.Error("left on an open folder should close it")
	}

	press(m, "left")

	if _, ok := m.top().(*labsPage); !ok {
		t.Fatalf("left on a closed folder at the top shows %T", m.top())
	}

	// Every folder at once, and back.
	press(m, "e")

	if !page.tbl.AllOpen() || !strings.Contains(ansi.Strip(m.View().Content), "ibgp") {
		t.Error("e did not open every folder")
	}

	press(m, "e")

	if strings.Contains(ansi.Strip(m.View().Content), "ibgp") {
		t.Error("a second e should close the folders again")
	}

	// A filter looks into the closed folders.
	press(m, "/")
	typeText(m, "ibgp")
	press(m, "enter")

	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "ibgp") ||
		strings.Contains(view, "ebgp") {
		t.Errorf("the filter should show ibgp alone in its folders:\n%s", view)
	}

	press(m, "esc")

	// The actions of a folder.
	page.tbl.SetCursorKey(folderKey("/home/demo/labs/training"))

	title, list := page.actions(m)
	if title != "training" || len(list) == 0 || list[0].key != keyFolders {
		t.Errorf("the actions of a folder are %q %+v", title, list)
	}
}

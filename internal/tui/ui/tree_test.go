// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

func treeColumns() []Column {
	return []Column{
		{ID: "name", Title: "NAME", Min: 4, Tree: true},
		{ID: "state", Title: "STATE"},
		{ID: "file", Title: "FILE", Flex: true},
	}
}

func treeLeaf(name, parent string) Row {
	return Row{Key: name, Parent: parent, Cells: []Cell{
		{Text: name}, {Text: "undeployed"}, {Text: name + ".clab.yml"},
	}}
}

func treeFolder(name, parent, count string) Row {
	return Row{Key: "dir:" + name, Parent: parent, Folder: true, Cells: []Cell{
		{Text: name}, {Text: count},
	}}
}

// newTreeTable returns a table of two labs at the top, dc1 and clos, and a folder named
// training holding the lab srl01 and the folder bgp, which holds ebgp and ibgp.
func newTreeTable(width, height int) *Table {
	t := NewTable(clabinternaltuitheme.Default(true), treeColumns())
	t.SetSize(width, height)
	t.SetRows([]Row{
		treeLeaf("dc1", ""),
		treeLeaf("clos", ""),
		treeFolder("training", "", "3 labs"),
		treeLeaf("srl01", "dir:training"),
		treeFolder("bgp", "dir:training", "2 labs"),
		treeLeaf("ebgp", "dir:bgp"),
		treeLeaf("ibgp", "dir:bgp"),
	})

	return t
}

func TestTreeFoldersStartClosed(t *testing.T) {
	tbl := newTreeTable(80, 12)

	if got, want := shownKeys(tbl), []string{"dc1", "clos", "dir:training"}; !slices.Equal(
		got, want,
	) {
		t.Fatalf("shown %v, want %v", got, want)
	}

	// The labs in closed folders count all the same: the folders do not.
	if shown, total := tbl.Counts(); shown != 5 || total != 5 {
		t.Errorf("counts %d/%d, want 5/5", shown, total)
	}

	view := ansi.Strip(tbl.View())
	if !strings.Contains(view, "▸ training  3 labs") {
		t.Errorf("the closed folder is not drawn with its chevron and count:\n%s", view)
	}
}

func TestTreeOpenAndClose(t *testing.T) {
	tbl := newTreeTable(80, 12)
	tbl.SetCursorKey("dir:training")

	if !tbl.ToggleFolder() {
		t.Fatal("the cursor is on a folder, it should open")
	}

	want := []string{"dc1", "clos", "dir:training", "srl01", "dir:bgp"}
	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Fatalf("shown %v, want %v", got, want)
	}

	view := ansi.Strip(tbl.View())
	for _, line := range []string{"▾ training", "  srl01", "  ▸ bgp  2 labs"} {
		if !strings.Contains(view, line) {
			t.Errorf("the open folder lacks %q:\n%s", line, view)
		}
	}

	// Closing a folder with the cursor inside puts the cursor on the folder.
	tbl.SetCursorKey("srl01")
	tbl.SetAllOpen(false)

	if got := tbl.CursorKey(); got != "dir:training" {
		t.Errorf("after closing, the cursor is on %q", got)
	}

	// One folder open is not all of them.
	if tbl.ToggleFolder(); tbl.AllOpen() {
		t.Error("bgp is closed, yet all folders count as open")
	}

	if tbl.SetAllOpen(true); !tbl.AllOpen() || !slices.Contains(shownKeys(tbl), "ebgp") {
		t.Errorf("after opening all folders %v are shown", shownKeys(tbl))
	}
}

// TestTreeWalk steps through the folders with the arrows, the way file trees do.
func TestTreeWalk(t *testing.T) {
	tbl := newTreeTable(80, 12)

	tbl.SetCursorKey("clos")

	if tbl.Walk("right") || tbl.Walk("left") {
		t.Error("on a lab at the top the arrows are left to the page")
	}

	steps := []struct {
		key, cursor string
		open        bool
	}{
		{"right", "dir:training", true}, // opens
		{"right", "srl01", true},        // steps in
		{"left", "dir:training", true},  // steps out
		{"left", "dir:training", false}, // closes
	}

	tbl.SetCursorKey("dir:training")
	tbl.SetAllOpen(false)

	for n, step := range steps {
		if !tbl.Walk(step.key) {
			t.Fatalf("step %d: %s was not used", n, step.key)
		}

		if got := tbl.CursorKey(); got != step.cursor {
			t.Fatalf("step %d: %s put the cursor on %q, want %q", n, step.key, got, step.cursor)
		}

		if _, open := tbl.CursorFolder(); step.cursor == "dir:training" && open != step.open {
			t.Fatalf("step %d: the folder is open=%v, want %v", n, open, step.open)
		}
	}
}

// TestTreeFilter finds labs in closed folders: the folders they are in show, open.
func TestTreeFilter(t *testing.T) {
	tbl := newTreeTable(80, 12)

	tbl.SetFilter("ibgp")

	want := []string{"dir:training", "dir:bgp", "ibgp"}
	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Fatalf("filter ibgp shows %v, want %v", got, want)
	}

	if shown, total := tbl.Counts(); shown != 1 || total != 5 {
		t.Errorf("counts %d/%d, want 1/5", shown, total)
	}

	// The cursor goes to the match, not to the folders around it.
	if got := tbl.CursorKey(); got != "ibgp" {
		t.Errorf("after filtering the cursor is on %q", got)
	}

	// A folder that matches shows everything in it.
	tbl.SetFilter("bgp")

	want = []string{"dir:training", "dir:bgp", "ebgp", "ibgp"}
	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Fatalf("filter bgp shows %v, want %v", got, want)
	}

	tbl.SetFilter("")

	if got := shownKeys(tbl); len(got) != 3 {
		t.Errorf("without the filter the folders are closed again, shown %v", got)
	}
}

// TestTreeSortKeepsFolders sorts within each folder: the labs stay in their folders, and
// the folders follow the labs.
func TestTreeSortKeepsFolders(t *testing.T) {
	tbl := newTreeTable(80, 12)
	tbl.SetAllOpen(true)
	tbl.SetSort("name", true)

	want := []string{"dc1", "clos", "dir:training", "srl01", "dir:bgp", "ibgp", "ebgp"}
	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Errorf("sorted by name descending: %v, want %v", got, want)
	}
}

// TestTreeRevealAndMarks: moving the cursor to a row in a closed folder opens it, and
// folders cannot be marked.
func TestTreeRevealAndMarks(t *testing.T) {
	tbl := newTreeTable(80, 12)

	if !tbl.SetCursorKey("ebgp") || tbl.CursorKey() != "ebgp" {
		t.Fatalf("the cursor did not reach a lab in a closed folder: %q", tbl.CursorKey())
	}

	tbl.SetCursorKey("dir:bgp")
	tbl.ToggleMark()
	tbl.ToggleMarkAll()

	for _, key := range tbl.Selection() {
		if strings.HasPrefix(key, "dir:") {
			t.Errorf("the folder %q was marked", key)
		}
	}
}

// TestTreeFolderNamesDoNotWidenColumns: the name of a folder may be long, it spans the
// row instead of making the column wide.
func TestTreeFolderNamesDoNotWidenColumns(t *testing.T) {
	tbl := newTreeTable(80, 12)
	before := strings.Split(ansi.Strip(tbl.View()), "\n")[0]

	rows := slices.Clone(tbl.rows)
	rows = append(rows, treeFolder("~/.clab/srl-labs/srl-telemetry-lab", "", "1 lab"),
		treeLeaf("st", "dir:~/.clab/srl-labs/srl-telemetry-lab"))
	tbl.SetRows(rows)

	if header := strings.Split(ansi.Strip(tbl.View()), "\n")[0]; header != before {
		t.Errorf("the name column grew with the folder:\n%q\n%q", before, header)
	}

	if view := ansi.Strip(tbl.View()); !strings.Contains(
		view, "▸ ~/.clab/srl-labs/srl-telemetry-lab  1 lab",
	) {
		t.Errorf("the long folder is not shown whole:\n%s", view)
	}
}

func TestTreeClickOnChevron(t *testing.T) {
	tbl := newTreeTable(80, 12)

	// The folder is the third row; its chevron is the first cell after the gutter.
	tbl.Click(tableGutter, 3)

	if _, open := tbl.CursorFolder(); !open {
		t.Error("a click on the chevron should open the folder")
	}

	tbl.Click(tableGutter+4, 3)

	if _, open := tbl.CursorFolder(); !open {
		t.Error("a click on the name only selects the folder")
	}
}

// TestTreeRowsInACycle: rows whose folders hold each other are never reached from the
// top and are left out, rather than walked forever.
func TestTreeRowsInACycle(t *testing.T) {
	tbl := NewTable(clabinternaltuitheme.Default(true), treeColumns())
	tbl.SetSize(80, 10)
	tbl.SetRows([]Row{
		treeLeaf("top", ""),
		treeFolder("a", "dir:b", ""),
		treeFolder("b", "dir:a", ""),
		treeLeaf("lost", "dir:a"),
		treeLeaf("orphan", "dir:missing"),
	})
	tbl.SetAllOpen(true)
	tbl.SetFilter("o")

	want := []string{"top", "orphan"}
	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Errorf("shown %v, want %v", got, want)
	}
}

// TestTreeFoldersCloseWhileFiltering: the filter opens the folders around its matches,
// and the keys still close and open them: none of them may do nothing.
func TestTreeFoldersCloseWhileFiltering(t *testing.T) {
	tbl := newTreeTable(80, 12)
	tbl.SetFilter("gp")
	tbl.SetCursorKey("dir:bgp")

	if !tbl.ToggleFolder() || slices.Contains(shownKeys(tbl), "ebgp") {
		t.Fatalf("enter on an open folder under a filter left %v", shownKeys(tbl))
	}

	if _, open := tbl.CursorFolder(); open {
		t.Error("the folder still counts as open")
	}

	if tbl.Walk("right"); !slices.Contains(shownKeys(tbl), "ebgp") {
		t.Fatalf("right did not open the folder again: %v", shownKeys(tbl))
	}

	tbl.SetCursorKey("dir:training")

	if !tbl.Walk("left") || slices.Contains(shownKeys(tbl), "dir:bgp") {
		t.Fatalf("left on an open folder under a filter left %v", shownKeys(tbl))
	}

	tbl.SetAllOpen(true)

	if !tbl.AllOpen() || !slices.Contains(shownKeys(tbl), "ibgp") {
		t.Fatalf("opening all under a filter shows %v", shownKeys(tbl))
	}

	tbl.SetAllOpen(false)

	if got := shownKeys(tbl); !slices.Equal(got, []string{"dir:training"}) {
		t.Fatalf("closing all under a filter shows %v", got)
	}

	// A new filter starts with the folders around its matches open again.
	tbl.SetFilter("ibgp")

	if got := shownKeys(tbl); !slices.Contains(got, "ibgp") {
		t.Errorf("a new filter shows %v", got)
	}

	// Without a filter the folders are as they were before it.
	tbl.SetFilter("")

	if got := shownKeys(tbl); len(got) != 3 {
		t.Errorf("without the filter %v are shown", got)
	}
}

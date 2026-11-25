// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

func testColumns() []Column {
	return []Column{
		{ID: "dot", Title: "", Max: 1, NoSort: true, NoFilter: true, Tight: true},
		{ID: "name", Title: "NAME", Min: 4},
		{ID: "kind", Title: "KIND", Priority: 1},
		{ID: "cpu", Title: "CPU", Priority: 2, Align: AlignRight, Numeric: true, NoFilter: true},
		{ID: "image", Title: "IMAGE", Min: 8, Flex: true, Priority: 3},
	}
}

func testRow(name, kind string, cpu float64, image string) Row {
	return Row{Key: name, Cells: []Cell{
		{Text: "●"},
		{Text: name},
		{Text: kind},
		{Text: fmt.Sprintf("%.0f%%", cpu), Num: cpu},
		{Text: image},
	}}
}

func testRows() []Row {
	return []Row{
		testRow("leaf10", "nokia_srlinux", 12, "ghcr.io/nokia/srlinux:25.10.1"),
		testRow("leaf2", "nokia_srlinux", 71, "ghcr.io/nokia/srlinux:25.10.1"),
		testRow("spine1", "nokia_srlinux", 9, "ghcr.io/nokia/srlinux:25.10.1"),
		testRow("client1", "linux", 100, "ghcr.io/srl-labs/network-multitool:latest"),
		testRow("leaf1", "arista_ceos", 3, "ceos:4.34.2F"),
	}
}

func newTestTable(width, height int) *Table {
	t := NewTable(clabinternaltuitheme.Default(true), testColumns())
	t.SetSize(width, height)
	t.SetRows(testRows())

	return t
}

func keyPress(name string) tea.KeyPressMsg {
	switch name {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "ctrl+a":
		return tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}
	default:
		code, _ := utf8.DecodeRuneInString(name)

		return tea.KeyPressMsg{Code: code, Text: name}
	}
}

func shownKeys(t *Table) []string {
	keys := make([]string, 0, len(t.shown))
	for _, i := range t.shown {
		keys = append(keys, t.rows[i].Key)
	}

	return keys
}

// TestTableNeverWraps renders the table at every size: it must return exactly the lines
// asked for, each exactly as wide as the table, whatever has to be cut or dropped.
func TestTableNeverWraps(t *testing.T) {
	for width := 1; width <= 160; width++ {
		for _, height := range []int{1, 2, 4, 12} {
			tbl := newTestTable(width, height)
			tbl.SetFilter("leaf")
			tbl.ToggleMark()

			lines := strings.Split(tbl.View(), "\n")
			if len(lines) != height {
				t.Fatalf("%dx%d: got %d lines", width, height, len(lines))
			}

			for i, line := range lines {
				if w := ansi.StringWidth(line); w != width {
					t.Fatalf(
						"%dx%d: line %d is %d cells wide: %q",
						width,
						height,
						i,
						w,
						ansi.Strip(line),
					)
				}
			}
		}
	}
}

func TestTableDropsColumnsByPriority(t *testing.T) {
	tests := []struct {
		width int
		want  []string
	}{
		{120, []string{"dot", "name", "kind", "cpu", "image"}},
		// The image column needs eight cells at the least, and its gap.
		{42, []string{"dot", "name", "kind", "cpu", "image"}},
		{41, []string{"dot", "name", "kind", "cpu"}},
		{32, []string{"dot", "name", "kind", "cpu"}},
		{31, []string{"dot", "name", "kind"}},
		{26, []string{"dot", "name", "kind"}},
		{25, []string{"dot", "name"}},
		// The columns without a priority stay even when they do not fit.
		{4, []string{"dot", "name"}},
	}

	for _, tt := range tests {
		tbl := newTestTable(tt.width, 8)
		if got := tbl.VisibleColumns(); !slices.Equal(got, tt.want) {
			t.Errorf("width %d: columns %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestTableFlexColumnGivesWayFirst(t *testing.T) {
	tbl := newTestTable(48, 8)
	view := ansi.Strip(tbl.View())

	if !strings.Contains(view, "nokia_srlinux") {
		t.Errorf("the fixed column was cut although a flexible one could shrink:\n%s", view)
	}

	if !strings.Contains(view, Ellipsis) {
		t.Errorf("expected the flexible column to be cut:\n%s", view)
	}
}

func TestTableFilter(t *testing.T) {
	tbl := newTestTable(100, 10)

	tbl.SetFilter("LEAF srl")

	if got, want := shownKeys(tbl), []string{"leaf10", "leaf2"}; !slices.Equal(got, want) {
		t.Errorf("all terms must match, ignoring case: got %v, want %v", got, want)
	}

	tbl.SetFilter("leaf !srl")

	if got, want := shownKeys(tbl), []string{"leaf1"}; !slices.Equal(got, want) {
		t.Errorf("a negated term must exclude: got %v, want %v", got, want)
	}

	// The CPU column is kept out of the filter text.
	tbl.SetFilter("100")

	if got := shownKeys(tbl); len(got) != 0 {
		t.Errorf("a column marked NoFilter matched: %v", got)
	}

	if view := ansi.Strip(tbl.View()); !strings.Contains(view, "nothing matches the filter") {
		t.Errorf("an empty result should say so:\n%s", view)
	}

	tbl.SetFilter("")

	if shown, total := tbl.Counts(); shown != total || total != 5 {
		t.Errorf("clearing the filter shows %d of %d rows", shown, total)
	}
}

func TestTableFilterHighlightsMatches(t *testing.T) {
	th := clabinternaltuitheme.Default(true)
	tbl := newTestTable(100, 10)
	tbl.Focused = false
	tbl.SetFilter("eaf")

	if view := tbl.View(); !strings.Contains(view, th.Match.Render("eaf")) {
		t.Error("the matched part of a value is not highlighted")
	}
}

func TestTableSort(t *testing.T) {
	tbl := newTestTable(100, 10)

	tbl.SetSort("name", false)

	want := []string{"client1", "leaf1", "leaf2", "leaf10", "spine1"}

	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Errorf("names sort naturally: got %v, want %v", got, want)
	}

	tbl.SetSort("cpu", true)

	want = []string{"client1", "leaf2", "leaf10", "spine1", "leaf1"}

	if got := shownKeys(tbl); !slices.Equal(got, want) {
		t.Errorf("numeric columns sort by value: got %v, want %v", got, want)
	}

	// Rows with equal values keep an ascending order by key in both directions.
	tbl.SetSort("kind", true)

	if got := shownKeys(tbl); !slices.Equal(got[:3], []string{"leaf2", "leaf10", "spine1"}) {
		t.Errorf("ties are not ordered by key: %v", got)
	}

	tbl.SetSort("unknown", false)

	if got := shownKeys(
		tbl,
	); !slices.Equal(
		got,
		[]string{"leaf10", "leaf2", "spine1", "client1", "leaf1"},
	) {
		t.Errorf("an unknown column should restore the given order: %v", got)
	}
}

func TestTableCycleSort(t *testing.T) {
	tbl := newTestTable(100, 10)

	tbl.HandleKey(keyPress(">"))

	if title, _, ok := tbl.SortState(); !ok || title != "NAME" {
		t.Errorf("the first sortable column should be picked, got %q", title)
	}

	tbl.HandleKey(keyPress(">"))
	tbl.HandleKey(keyPress("I"))

	if title, desc, _ := tbl.SortState(); title != "KIND" || !desc {
		t.Errorf("got sort %q desc=%v, want KIND descending", title, desc)
	}

	tbl.HandleKey(keyPress("<"))

	if title, desc, _ := tbl.SortState(); title != "NAME" || !desc {
		t.Errorf("got sort %q desc=%v, want NAME descending", title, desc)
	}
}

// TestTableCursorFollowsRow replaces the rows in a different order: the cursor must stay
// on the row it was on, not on the position.
func TestTableCursorFollowsRow(t *testing.T) {
	tbl := newTestTable(100, 10)
	tbl.SetCursorKey("spine1")

	rows := testRows()
	slices.Reverse(rows)
	tbl.SetRows(rows)

	if got := tbl.CursorKey(); got != "spine1" {
		t.Errorf("the cursor moved to %q", got)
	}

	// When the row is gone the cursor stays in range.
	tbl.SetRows(rows[:1])

	if got := tbl.CursorKey(); got != rows[0].Key {
		t.Errorf("the cursor is on %q after its row disappeared", got)
	}

	tbl.SetRows(nil)

	if _, ok := tbl.Cursor(); ok {
		t.Error("an empty table has no row under the cursor")
	}
}

func TestTableFilterResetsCursorUnlessRowStillMatches(t *testing.T) {
	tbl := newTestTable(100, 10)
	tbl.SetCursorKey("leaf2")

	tbl.SetFilter("leaf")

	if got := tbl.CursorKey(); got != "leaf2" {
		t.Errorf("the cursor left a row that still matches, now on %q", got)
	}

	tbl.SetFilter("spine")

	if got := tbl.CursorKey(); got != "spine1" {
		t.Errorf("a new query should start from the first match, cursor on %q", got)
	}
}

func TestTableMarks(t *testing.T) {
	tbl := newTestTable(100, 10)

	if got := tbl.Selection(); !slices.Equal(got, []string{"leaf10"}) {
		t.Errorf("without marks the selection is the cursor row, got %v", got)
	}

	tbl.HandleKey(keyPress("space"))
	tbl.HandleKey(keyPress("down"))
	tbl.HandleKey(keyPress("space"))

	if got := tbl.Selection(); !slices.Equal(got, []string{"leaf10", "spine1"}) {
		t.Errorf("got selection %v", got)
	}

	// Marks of rows that disappear are dropped.
	tbl.SetRows(testRows()[1:])

	if got := tbl.Selection(); !slices.Equal(got, []string{"spine1"}) {
		t.Errorf("after a row disappeared the selection is %v", got)
	}

	tbl.HandleKey(keyPress("ctrl+a"))

	if tbl.MarkCount() != 4 {
		t.Errorf("mark all marked %d rows, want 4", tbl.MarkCount())
	}

	tbl.HandleKey(keyPress("ctrl+a"))

	if tbl.MarkCount() != 0 {
		t.Errorf("mark all on a fully marked table should clear, %d left", tbl.MarkCount())
	}
}

func TestTableScrollsWithCursor(t *testing.T) {
	tbl := newTestTable(100, 4)

	tbl.HandleKey(keyPress("end"))
	view := ansi.Strip(tbl.View())

	if !strings.Contains(view, "leaf1 ") || strings.Contains(view, "leaf10") {
		t.Errorf("the view did not scroll to the last row:\n%s", view)
	}

	tbl.Scroll(-10)

	if got := tbl.CursorKey(); got != "leaf10" {
		t.Errorf("scrolling up should stop at the first row, cursor on %q", got)
	}
}

func TestTableClick(t *testing.T) {
	tbl := newTestTable(100, 10)

	if !tbl.Click(10, 3) || tbl.CursorKey() != "spine1" {
		t.Errorf("a click on the third row put the cursor on %q", tbl.CursorKey())
	}

	if tbl.Click(10, 9) {
		t.Error("a click below the last row should do nothing")
	}

	// A click on a column title sorts by it, a second one inverts the order.
	tbl.Click(5, 0)

	if title, desc, ok := tbl.SortState(); !ok || title != "NAME" || desc {
		t.Errorf("got sort %q desc=%v after a click on NAME", title, desc)
	}

	tbl.Click(5, 0)

	if _, desc, _ := tbl.SortState(); !desc {
		t.Error("a second click should invert the order")
	}
}

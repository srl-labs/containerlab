// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"slices"
	"sort"
	"strings"
)

const (
	// treeIndent is how far the rows of a folder are indented below it.
	treeIndent = 2
	// folderLead is the room in front of the name of a folder: the chevron and a space.
	// With a pixel layer there is a folder icon after the chevron, which takes two cells
	// more.
	folderLead      = 2
	pixelFolderLead = 4

	chevronClosed = "▸"
	chevronOpen   = "▾"
)

// indexTree finds out whether the rows form a tree, and if so which rows every folder
// holds and how deep each row is nested.
func (t *Table) indexTree() {
	t.tree = false
	t.kids = nil
	t.up = nil
	t.depth = nil
	t.leaves = 0

	folders := make(map[string]bool, len(t.rows))

	for i := range t.rows {
		if t.rows[i].Folder {
			folders[t.rows[i].Key] = true
		}

		if t.rows[i].Folder || t.rows[i].Parent != "" {
			t.tree = true
		}

		if !t.rows[i].Folder {
			t.leaves++
		}
	}

	if !t.tree {
		return
	}

	t.kids = map[string][]int{}
	t.up = map[string]string{}

	for i := range t.rows {
		parent := t.rows[i].Parent
		if !folders[parent] || parent == t.rows[i].Key {
			parent = ""
		}

		t.kids[parent] = append(t.kids[parent], i)

		if parent != "" {
			t.up[t.rows[i].Key] = parent
		}
	}

	// Rows are reached from the top only, so that rows whose folders hold each other
	// are left out instead of being walked forever.
	t.depth = make([]int, len(t.rows))
	t.walkTree("", func(i, depth int) bool {
		t.depth[i] = depth

		return true
	})
}

// walkTree calls fn for the rows in a folder and, where fn returns true, for the rows in
// the folders below, depth first and in the order the rows were given in.
func (t *Table) walkTree(parent string, fn func(i, depth int) bool) {
	var walk func(parent string, depth int)

	seen := make(map[string]bool, len(t.kids))
	walk = func(parent string, depth int) {
		if seen[parent] {
			return
		}

		seen[parent] = true

		for _, i := range t.kids[parent] {
			if fn(i, depth) && t.rows[i].Folder {
				walk(t.rows[i].Key, depth+1)
			}
		}
	}

	walk(parent, 0)
}

// rebuildTree lists the rows to show: the rows at the top, and below every open folder
// the rows in it. Siblings are sorted among each other, folders after the other rows.
// With a filter every folder is open and shows the rows that match, the folders that
// hold such rows, and everything in a folder that matches itself.
func (t *Table) rebuildTree() {
	filtering := len(t.terms) > 0
	self := make([]bool, len(t.rows))
	inside := make([]bool, len(t.rows))

	if filtering {
		var mark func(parent string) bool

		seen := map[string]bool{}
		mark = func(parent string) bool {
			if seen[parent] {
				return false
			}

			seen[parent] = true
			found := false

			for _, i := range t.kids[parent] {
				self[i] = t.matches(&t.rows[i])
				inside[i] = t.rows[i].Folder && mark(t.rows[i].Key)
				found = found || self[i] || inside[i]
			}

			return found
		}

		mark("")
	}

	t.leavesShown = 0

	var walk func(parent string, matched, visible bool)

	seen := map[string]bool{}
	walk = func(parent string, matched, visible bool) {
		if seen[parent] {
			return
		}

		seen[parent] = true

		for _, i := range t.siblings(parent) {
			row := &t.rows[i]
			if filtering && !matched && !self[i] && !inside[i] {
				continue
			}

			if visible {
				t.shown = append(t.shown, i)
			}

			if !row.Folder {
				t.leavesShown++

				continue
			}

			walk(row.Key, matched || self[i], visible && t.folderOpen(row))
		}
	}

	walk("", false, true)
}

// siblings returns the rows in a folder in the order they are shown.
func (t *Table) siblings(parent string) []int {
	kids := t.kids[parent]
	if t.sortCol < 0 || t.sortCol >= len(t.cols) {
		return kids
	}

	kids = slices.Clone(kids)
	sort.SliceStable(kids, func(a, b int) bool {
		if fa, fb := t.rows[kids[a]].Folder, t.rows[kids[b]].Folder; fa != fb {
			return fb
		}

		return t.less(kids[a], kids[b])
	})

	return kids
}

// spans reports whether the cell of a row in a column is part of the span of a folder,
// see Row.Folder.
func (t *Table) spans(i, col int) bool {
	if !t.tree || !t.rows[i].Folder {
		return false
	}

	for c := range col + 1 {
		if t.cols[c].Tree {
			return true
		}
	}

	return false
}

// indent returns the width the tree column puts in front of the text of a row. The rows
// of a folder line up with the name of the folder.
func (t *Table) indent(i int) int {
	if !t.tree || i >= len(t.depth) {
		return 0
	}

	lead := folderLead
	if t.pixels {
		lead = pixelFolderLead
	}

	depth := t.depth[i]

	switch {
	case t.rows[i].Folder:
		return depth*treeIndent + lead
	case depth == 0:
		return 0
	default:
		return (depth-1)*treeIndent + lead
	}
}

// lead returns what the tree column shows in front of the text of a row: its indentation,
// and for a folder the chevron. With a pixel layer the chevron and the folder icon are
// drawn there, and the text leaves room for them.
func (t *Table) lead(i int) string {
	width := t.indent(i)
	if width == 0 {
		return ""
	}

	if !t.rows[i].Folder || t.pixels {
		return strings.Repeat(" ", width)
	}

	chevron := chevronClosed
	if t.folderOpen(&t.rows[i]) {
		chevron = chevronOpen
	}

	pad := strings.Repeat(" ", t.depth[i]*treeIndent)

	return pad + chevron + strings.Repeat(" ", width-len(pad)-1)
}

// folderOpen reports whether a folder is shown open: it is, or a filter is set and the
// folder was not closed since.
func (t *Table) folderOpen(r *Row) bool {
	if !r.Folder {
		return false
	}

	if len(t.terms) > 0 {
		return !t.shut[r.Key]
	}

	return t.open[r.Key]
}

// filtering reports whether a filter is set, under which the folders open on their own.
func (t *Table) filtering() bool { return len(t.terms) > 0 }

// chevronSpan returns the cells, from the left edge of the table, that open and close the
// folder in a row: its chevron, and with a pixel layer its icon too.
func (t *Table) chevronSpan(i int) (from, to int, ok bool) {
	if !t.tree || !t.rows[i].Folder {
		return 0, 0, false
	}

	t.ensureLayout()

	pos := t.gutter()

	for k, c := range t.layout.cols {
		if k > 0 {
			pos += t.gapAfter(t.layout.cols[k-1])
		}

		if t.cols[c].Tree {
			from = pos + t.depth[i]*treeIndent

			return from, pos + t.indent(i) - 1, true
		}

		pos += t.layout.widths[k]
	}

	return 0, 0, false
}

// parentOf returns the key of the folder the row with the given key is in, empty when it
// is at the top or not known.
func (t *Table) parentOf(key string) string {
	return t.up[key]
}

// reveal opens the folders the row with the given key is in and reports whether it
// opened any.
func (t *Table) reveal(key string) bool {
	opened := false
	seen := map[string]bool{}

	for parent := t.parentOf(key); parent != "" && !seen[parent]; parent = t.parentOf(parent) {
		seen[parent] = true

		switch {
		case t.filtering() && t.shut[parent]:
			delete(t.shut, parent)
		case !t.filtering() && !t.open[parent]:
			t.open[parent] = true
		default:
			continue
		}

		opened = true
	}

	return opened
}

// setOpen opens or closes a folder. When the cursor was in a folder that closes, it moves
// to the folder. Under a filter it only lasts as long as the filter.
func (t *Table) setOpen(key string, open bool) {
	cursor := t.CursorKey()

	switch {
	case t.filtering() && open:
		delete(t.shut, key)
	case t.filtering():
		t.shut[key] = true
	case open:
		t.open[key] = true
	default:
		delete(t.open, key)
	}

	t.forget()
	t.rebuild()
	t.restoreCursor(cursor)
}

// HasFolders reports whether some rows are folders.
func (t *Table) HasFolders() bool {
	return t.tree && t.leaves < len(t.rows)
}

// CursorFolder reports whether the cursor is on a folder, and whether that is open.
func (t *Table) CursorFolder() (folder, open bool) {
	r, ok := t.Cursor()
	if !ok || !r.Folder {
		return false, false
	}

	return true, t.folderOpen(r)
}

// ToggleFolder opens the folder under the cursor, or closes it when it is open. It
// reports false when the cursor is not on a folder.
func (t *Table) ToggleFolder() bool {
	r, ok := t.Cursor()
	if !ok || !r.Folder {
		return false
	}

	t.setOpen(r.Key, !t.folderOpen(r))

	return true
}

// AllOpen reports whether every folder is open.
func (t *Table) AllOpen() bool {
	for i := range t.rows {
		if t.rows[i].Folder && !t.folderOpen(&t.rows[i]) {
			return false
		}
	}

	return true
}

// SetAllOpen opens or closes every folder. Under a filter it only lasts as long as the
// filter.
func (t *Table) SetAllOpen(open bool) {
	cursor := t.CursorKey()

	set := t.open
	if t.filtering() {
		set, open = t.shut, !open
	}

	clear(set)

	if open {
		for i := range t.rows {
			if t.rows[i].Folder {
				set[t.rows[i].Key] = true
			}
		}
	}

	t.forget()
	t.rebuild()
	t.restoreCursor(cursor)
}

// Walk moves through the folders with the arrow keys, the way file trees do: right opens
// the folder under the cursor, or steps into it when it is open; left closes it, or steps
// out of the folder the row is in. It reports whether it used the key, which it does not
// on a row that is no folder for right, and on a row at the top for left: the arrows
// then do what they do elsewhere.
func (t *Table) Walk(key string) bool {
	r, ok := t.Cursor()
	if !ok || !t.tree {
		return false
	}

	switch key {
	case "right":
		if !r.Folder {
			return false
		}

		if !t.folderOpen(r) {
			t.setOpen(r.Key, true)

			return true
		}

		if t.cursor+1 < len(t.shown) && t.rows[t.shown[t.cursor+1]].Parent == r.Key {
			t.Move(1)
		}

		return true
	case "left":
		if t.folderOpen(r) {
			t.setOpen(r.Key, false)

			return true
		}

		parent := t.parentOf(r.Key)

		return parent != "" && t.moveTo(parent)
	}

	return false
}

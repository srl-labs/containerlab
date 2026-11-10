// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ui

import (
	"sort"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

const (
	// tableGutter is the width of the cursor bar and the mark in front of every row.
	tableGutter = 2
	// tableGap separates two columns.
	tableGap = 2
	// wheelStep is the number of rows a mouse wheel notch scrolls.
	wheelStep = 3

	cursorBar = "▌"
	markSign  = "✓"
	// scrollThumb marks the part of a long table that is in view, at its right edge.
	scrollThumb = "▐"
	sortUp      = "▲"
	sortDown    = "▼"
)

// Align is the horizontal alignment of a column.
type Align int

// Column alignments.
const (
	AlignLeft Align = iota
	AlignRight
)

// Column describes a table column.
type Column struct {
	ID    string
	Title string
	// Min and Max bound the width of the column, a Max of zero means no upper bound.
	Min int
	Max int
	// Flex columns give up width first when the table does not fit, down to Min.
	Flex bool
	// Priority decides which columns are dropped on narrow terminals: columns with the
	// highest value go first, the ones with a priority of zero always stay.
	Priority int
	Align    Align
	// TruncLeft cuts long values at their start instead of their end.
	TruncLeft bool
	// Numeric columns sort by the Num of their cells instead of the text.
	Numeric bool
	// NoFilter keeps the column out of the text the filter is matched against.
	NoFilter bool
	NoSort   bool
	// Tight columns are followed by a single space instead of the regular gap. It suits
	// one cell wide columns holding a state glyph.
	Tight bool
}

// Cell is the value of a row in one column.
type Cell struct {
	Text  string
	Style lipgloss.Style
	// Num is the sort value of the cell in a numeric column.
	Num float64
}

// Row is a table row. Its key identifies it across updates, which is what keeps the cursor
// and the marks on the same item while the rows change underneath.
type Row struct {
	Key   string
	Cells []Cell
}

// Table is a scrolling, sortable and filterable table with a cursor and row marks. It
// never wraps: columns shrink or drop out until the rows fit the width.
type Table struct {
	th   *clabinternaltuitheme.Theme
	cols []Column
	rows []Row
	// shown lists the rows that pass the filter, in display order.
	shown []int

	cursor int
	offset int
	marks  map[string]struct{}

	width  int
	height int

	filter string
	terms  []filterTerm

	sortCol  int
	sortDesc bool

	natural []int
	layout  tableLayout
	stale   bool
	// scrollbar is set while the table has more rows than fit, which takes one column
	// at the right edge for the scrollbar.
	scrollbar bool

	// drawn holds the rows as last rendered, by key. Styling a row is what drawing a
	// table costs, and most rows look the same from one frame to the next: moving the
	// cursor changes two of them.
	drawn map[string]drawnRow

	// Empty is shown in place of the rows when there are none.
	Empty string
	// Focused draws the cursor row highlighted. Tables sharing a screen clear it on the
	// ones that do not have the keyboard.
	Focused bool
}

type tableLayout struct {
	cols   []int
	widths []int
}

// drawnRow is a rendered row and the state it was rendered in.
type drawnRow struct {
	line  string
	state rowState
}

// rowState is what, besides its cells, decides how a row looks.
type rowState struct {
	cursor, focused, marked bool
}

// forget drops the rendered rows. It is called whenever something changes that affects
// how every row looks: the rows themselves, the layout, the filter or the theme.
func (t *Table) forget() {
	clear(t.drawn)
}

// gapAfter returns the space that follows a column.
func (t *Table) gapAfter(col int) int {
	if t.cols[col].Tight {
		return 1
	}

	return tableGap
}

// gapsWidth returns the total width of the gaps between the given columns.
func (t *Table) gapsWidth(cols []int) int {
	w := 0

	for k := 0; k+1 < len(cols); k++ {
		w += t.gapAfter(cols[k])
	}

	return w
}

type filterTerm struct {
	text   string
	negate bool
}

var emptyCell Cell

// NewTable creates a table with the given columns.
func NewTable(th *clabinternaltuitheme.Theme, cols []Column) *Table {
	return &Table{
		th:      th,
		cols:    cols,
		marks:   map[string]struct{}{},
		drawn:   map[string]drawnRow{},
		sortCol: -1,
		stale:   true,
		Focused: true,
	}
}

// SetTheme changes the theme the table is drawn with.
func (t *Table) SetTheme(th *clabinternaltuitheme.Theme) {
	t.th = th
	t.forget()
}

// SetSize sets the size of the table, header included.
func (t *Table) SetSize(width, height int) {
	if width == t.width && height == t.height {
		return
	}

	t.width, t.height = width, height
	t.stale = true
	t.clamp()
}

// SetRows replaces the rows. The cursor stays on the row it was on, by key.
func (t *Table) SetRows(rows []Row) {
	key := t.CursorKey()

	t.rows = rows
	t.forget()
	t.measure()
	t.rebuild()
	t.restoreCursor(key)
	t.pruneMarks()
}

// measure computes the natural width of every column: wide enough for its title and its
// widest value, within the column's bounds.
func (t *Table) measure() {
	t.natural = make([]int, len(t.cols))

	for i := range t.cols {
		w := Width(t.cols[i].Title)

		for r := range t.rows {
			w = max(w, Width(t.cell(&t.rows[r], i).Text))
		}

		w = max(w, t.cols[i].Min, 1)
		if t.cols[i].Max > 0 {
			w = min(w, t.cols[i].Max)
		}

		t.natural[i] = w
	}

	t.stale = true
}

func (t *Table) cell(r *Row, col int) *Cell {
	if col < len(r.Cells) {
		return &r.Cells[col]
	}

	return &emptyCell
}

// rebuild applies the filter and the sort order.
func (t *Table) rebuild() {
	t.shown = t.shown[:0]

	for i := range t.rows {
		if t.matches(&t.rows[i]) {
			t.shown = append(t.shown, i)
		}
	}

	if t.sortCol >= 0 && t.sortCol < len(t.cols) {
		sort.SliceStable(t.shown, func(a, b int) bool {
			return t.less(t.shown[a], t.shown[b])
		})
	}

	t.clamp()
}

func (t *Table) less(a, b int) bool {
	ca, cb := t.cell(&t.rows[a], t.sortCol), t.cell(&t.rows[b], t.sortCol)

	var c int

	switch {
	case t.cols[t.sortCol].Numeric:
		switch {
		case ca.Num < cb.Num:
			c = -1
		case ca.Num > cb.Num:
			c = 1
		}
	default:
		c = naturalCompare(ca.Text, cb.Text)
	}

	if c == 0 {
		// Ties keep a stable, ascending order whatever the direction of the sort.
		return naturalCompare(t.rows[a].Key, t.rows[b].Key) < 0
	}

	if t.sortDesc {
		return c > 0
	}

	return c < 0
}

// SetFilter sets the filter: space separated terms that must all occur in a row, in any
// column and ignoring case. A term starting with an exclamation mark must not occur.
func (t *Table) SetFilter(query string) {
	if query == t.filter {
		return
	}

	key := t.CursorKey()

	t.filter = query
	t.terms = parseFilter(query)
	t.forget()
	t.rebuild()

	// A new query starts from the first match, unless the row under the cursor still
	// matches, which keeps the selection steady while the query is refined.
	t.cursor, t.offset = 0, 0
	t.restoreCursor(key)
}

// Filter returns the current filter.
func (t *Table) Filter() string { return t.filter }

func parseFilter(query string) []filterTerm {
	fields := strings.Fields(strings.ToLower(query))
	terms := make([]filterTerm, 0, len(fields))

	for _, f := range fields {
		if rest, negated := strings.CutPrefix(f, "!"); negated {
			if rest != "" {
				terms = append(terms, filterTerm{text: rest, negate: true})
			}

			continue
		}

		terms = append(terms, filterTerm{text: f})
	}

	return terms
}

func (t *Table) matches(r *Row) bool {
	for _, term := range t.terms {
		found := false

		for i := range t.cols {
			if t.cols[i].NoFilter {
				continue
			}

			if strings.Contains(strings.ToLower(t.cell(r, i).Text), term.text) {
				found = true

				break
			}
		}

		if found == term.negate {
			return false
		}
	}

	return true
}

// SetSort sorts the table by the column with the given ID. An unknown ID restores the
// order the rows were given in.
func (t *Table) SetSort(id string, desc bool) {
	key := t.CursorKey()

	t.sortCol, t.sortDesc = -1, desc

	for i := range t.cols {
		if t.cols[i].ID == id && !t.cols[i].NoSort {
			t.sortCol = i

			break
		}
	}

	t.rebuild()
	t.restoreCursor(key)
}

// CycleSort moves the sort to the next (or with a negative delta the previous) visible
// column that can be sorted.
func (t *Table) CycleSort(delta int) {
	t.ensureLayout()

	var sortable []int

	for _, c := range t.layout.cols {
		if !t.cols[c].NoSort {
			sortable = append(sortable, c)
		}
	}

	if len(sortable) == 0 {
		return
	}

	pos := 0

	for i, c := range sortable {
		if c == t.sortCol {
			pos = (i + delta + len(sortable)) % len(sortable)

			break
		}
	}

	t.SetSort(t.cols[sortable[pos]].ID, t.sortDesc)
}

// InvertSort flips the direction of the sort.
func (t *Table) InvertSort() {
	if t.sortCol < 0 {
		return
	}

	t.SetSort(t.cols[t.sortCol].ID, !t.sortDesc)
}

// SortState returns the title of the column the table is sorted by and the direction.
func (t *Table) SortState() (title string, desc, sorted bool) {
	if t.sortCol < 0 {
		return "", false, false
	}

	return t.cols[t.sortCol].Title, t.sortDesc, true
}

// Counts returns the number of rows shown and the total number of rows.
func (t *Table) Counts() (shown, total int) {
	return len(t.shown), len(t.rows)
}

// Cursor returns the row under the cursor.
func (t *Table) Cursor() (*Row, bool) {
	if t.cursor < 0 || t.cursor >= len(t.shown) {
		return nil, false
	}

	return &t.rows[t.shown[t.cursor]], true
}

// CursorKey returns the key of the row under the cursor, empty when there is none.
func (t *Table) CursorKey() string {
	if r, ok := t.Cursor(); ok {
		return r.Key
	}

	return ""
}

// SetCursorKey moves the cursor to the row with the given key, if it is shown.
func (t *Table) SetCursorKey(key string) bool {
	for i, idx := range t.shown {
		if t.rows[idx].Key == key {
			t.cursor = i
			t.clamp()

			return true
		}
	}

	return false
}

func (t *Table) restoreCursor(key string) {
	if key != "" && t.SetCursorKey(key) {
		return
	}

	t.clamp()
}

// Move moves the cursor by delta rows.
func (t *Table) Move(delta int) {
	t.cursor += delta
	t.clamp()
}

func (t *Table) bodyHeight() int {
	return max(0, t.height-1)
}

func (t *Table) clamp() {
	t.cursor = max(0, min(t.cursor, len(t.shown)-1))

	body := t.bodyHeight()
	if t.cursor < t.offset {
		t.offset = t.cursor
	}

	if body > 0 && t.cursor >= t.offset+body {
		t.offset = t.cursor - body + 1
	}

	t.offset = max(0, min(t.offset, len(t.shown)-body))
}

// ToggleMark marks or unmarks the row under the cursor.
func (t *Table) ToggleMark() {
	r, ok := t.Cursor()
	if !ok {
		return
	}

	if _, marked := t.marks[r.Key]; marked {
		delete(t.marks, r.Key)

		return
	}

	t.marks[r.Key] = struct{}{}
}

// ToggleMarkAll marks every shown row, or clears the marks when they all are already.
func (t *Table) ToggleMarkAll() {
	all := len(t.shown) > 0

	for _, idx := range t.shown {
		if _, marked := t.marks[t.rows[idx].Key]; !marked {
			all = false

			break
		}
	}

	if all {
		t.ClearMarks()

		return
	}

	for _, idx := range t.shown {
		t.marks[t.rows[idx].Key] = struct{}{}
	}
}

// ClearMarks removes every mark.
func (t *Table) ClearMarks() {
	t.marks = map[string]struct{}{}
}

// MarkCount returns the number of marked rows.
func (t *Table) MarkCount() int { return len(t.marks) }

// Selection returns the keys the next action applies to: the marked rows in display
// order, or the row under the cursor when nothing is marked.
func (t *Table) Selection() []string {
	if len(t.marks) == 0 {
		if key := t.CursorKey(); key != "" {
			return []string{key}
		}

		return nil
	}

	keys := make([]string, 0, len(t.marks))

	for i := range t.rows {
		if _, marked := t.marks[t.rows[i].Key]; marked {
			keys = append(keys, t.rows[i].Key)
		}
	}

	return keys
}

func (t *Table) pruneMarks() {
	if len(t.marks) == 0 {
		return
	}

	present := make(map[string]struct{}, len(t.rows))
	for i := range t.rows {
		present[t.rows[i].Key] = struct{}{}
	}

	for key := range t.marks {
		if _, ok := present[key]; !ok {
			delete(t.marks, key)
		}
	}
}

// HandleKey applies a navigation or marking key and reports whether it was one.
func (t *Table) HandleKey(msg tea.KeyPressMsg) bool {
	page := max(1, t.bodyHeight()-1)

	switch msg.String() {
	case "up", "k":
		t.Move(-1)
	case "down", "j":
		t.Move(1)
	case "pgup", "ctrl+b":
		t.Move(-page)
	case "pgdown", "ctrl+f":
		t.Move(page)
	case "ctrl+u":
		t.Move(-max(1, page/2)) //nolint:mnd // half a page
	case "ctrl+d":
		t.Move(max(1, page/2)) //nolint:mnd // half a page
	case "home", "g":
		t.Move(-len(t.shown))
	case "end", "G":
		t.Move(len(t.shown))
	case "space":
		t.ToggleMark()
		t.Move(1)
	case "ctrl+a":
		t.ToggleMarkAll()
	case ">", "shift+right":
		t.CycleSort(1)
	case "<", "shift+left":
		t.CycleSort(-1)
	case "I", "shift+up", "shift+down":
		t.InvertSort()
	default:
		return false
	}

	return true
}

// Scroll moves the cursor by a number of mouse wheel notches.
func (t *Table) Scroll(notches int) {
	t.Move(notches * wheelStep)
}

// Click handles a mouse click at a position relative to the table's top left corner. A
// click on a row moves the cursor there, one on a column title sorts by that column.
func (t *Table) Click(x, y int) bool {
	if y == 0 {
		return t.clickHeader(x)
	}

	i := t.offset + y - 1
	if y < 0 || i >= len(t.shown) {
		return false
	}

	t.cursor = i
	t.clamp()

	return true
}

func (t *Table) clickHeader(x int) bool {
	t.ensureLayout()

	pos := tableGutter

	for k, c := range t.layout.cols {
		span := t.layout.widths[k] + t.gapAfter(c)

		if x >= pos && x < pos+span {
			if t.cols[c].NoSort {
				return false
			}

			t.SetSort(t.cols[c].ID, c == t.sortCol && !t.sortDesc)

			return true
		}

		pos += span
	}

	return false
}

func (t *Table) ensureLayout() {
	if !t.stale {
		return
	}

	t.stale = false
	t.layout = t.computeLayout()
	t.forget()
}

// computeLayout fits the columns into the width. Columns are dropped by priority until
// the remaining ones fit at their smallest width; what space is left then goes to the
// flexible columns, the narrowest first so that short values are not cut needlessly.
func (t *Table) computeLayout() tableLayout {
	if len(t.natural) != len(t.cols) {
		t.measure()
		t.stale = false
	}

	visible := make([]int, len(t.cols))
	for i := range t.cols {
		visible[i] = i
	}

	avail := t.rowWidth() - tableGutter

	for t.minWidth(visible) > avail {
		drop := -1

		for k, c := range visible {
			if t.cols[c].Priority > 0 &&
				(drop < 0 || t.cols[c].Priority >= t.cols[visible[drop]].Priority) {
				drop = k
			}
		}

		if drop < 0 {
			break
		}

		visible = append(visible[:drop], visible[drop+1:]...)
	}

	widths := make([]int, len(visible))
	free := avail - t.gapsWidth(visible)

	var flex []int

	for k, c := range visible {
		if t.cols[c].Flex {
			flex = append(flex, k)

			continue
		}

		widths[k] = t.natural[c]
		free -= widths[k]
	}

	sort.SliceStable(flex, func(a, b int) bool {
		return t.natural[visible[flex[a]]] < t.natural[visible[flex[b]]]
	})

	for n, k := range flex {
		c := visible[k]
		w := min(t.natural[c], free/(len(flex)-n))
		w = max(w, t.cols[c].Min, 1)
		widths[k] = w
		free -= w
	}

	return tableLayout{cols: visible, widths: widths}
}

// minWidth returns the width the given columns need at the very least.
func (t *Table) minWidth(cols []int) int {
	w := t.gapsWidth(cols)

	for _, c := range cols {
		if t.cols[c].Flex {
			w += max(t.cols[c].Min, 1)

			continue
		}

		w += t.natural[c]
	}

	return w
}

// rowWidth is the width of a row: the table less the scrollbar, when there is one.
func (t *Table) rowWidth() int {
	if t.scrollbar {
		return max(1, t.width-1)
	}

	return t.width
}

// syncScrollbar shows the scrollbar when the rows do not fit and hides it when they do.
// It takes a column, so the layout is worked out again when that changes.
func (t *Table) syncScrollbar() {
	need := t.width > tableGutter+1 && len(t.shown) > t.bodyHeight() && t.bodyHeight() > 0
	if need != t.scrollbar {
		t.scrollbar = need
		t.stale = true
	}
}

// thumb returns the rows of the body the scrollbar thumb covers.
func (t *Table) thumb() (from, to int) {
	body, total := t.bodyHeight(), len(t.shown)
	if !t.scrollbar || total <= body {
		return 0, 0
	}

	size := max(1, body*body/total)
	from = t.offset * (body - size) / (total - body)

	return from, from + size
}

// VisibleColumns returns the IDs of the columns that fit the current width.

func (t *Table) VisibleColumns() []string {
	t.ensureLayout()

	ids := make([]string, 0, len(t.layout.cols))
	for _, c := range t.layout.cols {
		ids = append(ids, t.cols[c].ID)
	}

	return ids
}

// View renders the table as exactly the configured number of lines, each exactly as wide
// as the table.
func (t *Table) View() string {
	if t.width <= 0 || t.height <= 0 {
		return ""
	}

	t.syncScrollbar()
	t.ensureLayout()

	lines := make([]string, 0, t.height)
	lines = append(lines, t.renderHeader())

	body := t.bodyHeight()

	if len(t.shown) == 0 && body > 0 {
		lines = append(lines, t.renderEmpty(body)...)
	}

	for i := t.offset; i < min(len(t.shown), t.offset+body); i++ {
		lines = append(lines, t.renderRow(i))
	}

	blank := strings.Repeat(" ", t.rowWidth())
	for len(lines) < t.height {
		lines = append(lines, blank)
	}

	lines = lines[:t.height]

	if t.scrollbar {
		from, to := t.thumb()

		for i := range lines {
			bar := " "
			if row := i - 1; row >= from && row < to {
				bar = t.th.Muted.Render(scrollThumb)
			}

			lines[i] += bar
		}
	}

	return strings.Join(lines, "\n")
}

func (t *Table) renderEmpty(body int) []string {
	text := t.Empty
	if t.filter != "" && len(t.rows) > 0 {
		text = "nothing matches the filter"
	}

	if text == "" {
		return nil
	}

	blank := strings.Repeat(" ", t.rowWidth())
	lines := make([]string, 0, body)

	const emptyTextOffset = 1 // one blank line between the header and the text

	for len(lines) < min(emptyTextOffset, body-1) {
		lines = append(lines, blank)
	}

	return append(lines, t.th.Muted.Render(Center(text, t.rowWidth())))
}

func (t *Table) renderHeader() string {
	var b strings.Builder

	b.WriteString(strings.Repeat(" ", tableGutter))

	used := tableGutter

	for k, c := range t.layout.cols {
		if k > 0 {
			gap := t.gapAfter(t.layout.cols[k-1])
			b.WriteString(strings.Repeat(" ", gap))

			used += gap
		}

		w := t.layout.widths[k]
		title := t.cols[c].Title
		style := t.th.TableHeader

		if c == t.sortCol {
			style = t.th.Title

			if Width(title) < w {
				arrow := sortUp
				if t.sortDesc {
					arrow = sortDown
				}

				title += arrow
			}
		}

		if t.cols[c].Align == AlignRight {
			title = PadLeft(title, w)
		} else {
			title = PadRight(title, w)
		}

		b.WriteString(style.Render(title))

		used += w
	}

	return t.fit(b.String(), used, lipgloss.NewStyle())
}

// renderRow returns the row at a position of the display order, from the rows drawn
// before when it still looks the same.
func (t *Table) renderRow(i int) string {
	row := &t.rows[t.shown[i]]
	_, marked := t.marks[row.Key]
	state := rowState{cursor: i == t.cursor, focused: t.Focused, marked: marked}

	if d, ok := t.drawn[row.Key]; ok && d.state == state {
		return d.line
	}

	line := t.drawRow(i, row, marked)
	t.drawn[row.Key] = drawnRow{line: line, state: state}

	return line
}

func (t *Table) drawRow(i int, row *Row, marked bool) string {
	selected := i == t.cursor && t.Focused

	fill := lipgloss.NewStyle()
	if selected {
		fill = fill.Background(t.th.Selection)
	}

	var b strings.Builder

	bar, mark := " ", " "
	if i == t.cursor {
		bar = cursorBar
	}

	if marked {
		mark = markSign
	}

	b.WriteString(t.th.Accent.Inherit(fill).Render(bar))
	b.WriteString(t.th.Mark.Inherit(fill).Render(mark))

	used := tableGutter

	for k, c := range t.layout.cols {
		if k > 0 {
			gap := t.gapAfter(t.layout.cols[k-1])
			b.WriteString(fill.Render(strings.Repeat(" ", gap)))

			used += gap
		}

		b.WriteString(t.renderCell(row, c, t.layout.widths[k], selected, fill))

		used += t.layout.widths[k]
	}

	return t.fit(b.String(), used, fill)
}

func (t *Table) renderCell(row *Row, col, width int, selected bool, fill lipgloss.Style) string {
	c := t.cell(row, col)

	text := c.Text
	if t.cols[col].TruncLeft {
		text = TruncateLeft(text, width)
	} else {
		text = Truncate(text, width)
	}

	style, match := c.Style, t.th.Match
	if selected {
		style = style.Inherit(t.th.Selected)
		match = match.Inherit(fill)
	}

	var content string

	if len(t.terms) > 0 && !t.cols[col].NoFilter {
		content = highlight(text, t.terms, style, match)
	} else {
		content = style.Render(text)
	}

	pad := fill.Render(strings.Repeat(" ", max(0, width-Width(text))))

	if t.cols[col].Align == AlignRight {
		return pad + content
	}

	return content + pad
}

// fit pads a rendered line to the table width, or cuts it when even the columns that
// cannot be dropped do not fit.
func (t *Table) fit(line string, used int, fill lipgloss.Style) string {
	width := t.rowWidth()

	switch {
	case used < width:
		return line + fill.Render(strings.Repeat(" ", width-used))
	case used > width:
		return Truncate(line, width)
	default:
		return line
	}
}

// highlight renders text with every occurrence of the filter terms in the match style.
func highlight(text string, terms []filterTerm, style, match lipgloss.Style) string {
	runes := []rune(text)
	lower := make([]rune, len(runes))

	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}

	marked := make([]bool, len(runes))
	hit := false

	for _, term := range terms {
		if term.negate {
			continue
		}

		needle := []rune(term.text)

		for at := indexRunes(lower, needle, 0); at >= 0; at = indexRunes(lower, needle, at+1) {
			for k := range needle {
				marked[at+k] = true
			}

			hit = true
		}
	}

	if !hit {
		return style.Render(text)
	}

	var b strings.Builder

	for start := 0; start < len(runes); {
		end := start
		for end < len(runes) && marked[end] == marked[start] {
			end++
		}

		if marked[start] {
			b.WriteString(match.Render(string(runes[start:end])))
		} else {
			b.WriteString(style.Render(string(runes[start:end])))
		}

		start = end
	}

	return b.String()
}

// indexRunes returns the index of the first occurrence of needle in haystack at or after
// from, or -1.
func indexRunes(haystack, needle []rune, from int) int {
	if len(needle) == 0 {
		return -1
	}

	for i := from; i+len(needle) <= len(haystack); i++ {
		found := true

		for k := range needle {
			if haystack[i+k] != needle[k] {
				found = false

				break
			}
		}

		if found {
			return i
		}
	}

	return -1
}

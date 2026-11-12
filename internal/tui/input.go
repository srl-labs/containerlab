// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"fmt"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

const (
	keyEnter = "enter"
	keyEsc   = "esc"
	keyTab   = "tab"

	// bodyTop is the screen line the body starts at, below the header and the rule.
	bodyTop = 2

	// doubleClick is the longest pause between two clicks that count as a double click.
	doubleClick = 400 * time.Millisecond
)

// overlay is a dialog drawn on top of the page. While one is open it gets every key.
type overlay interface {
	// key handles a key press and reports whether the dialog is done.
	key(m *Model, msg tea.KeyPressMsg) (closed bool, cmd tea.Cmd)
	update(m *Model, msg tea.Msg) tea.Cmd
	view(m *Model, maxWidth, maxHeight int) string
	hints() []hint
	paste(text string)
	// click handles a mouse click at a position relative to the top left corner of the
	// dialog and reports whether the dialog is done.
	click(m *Model, x, y int) (closed bool, cmd tea.Cmd)
	// scroll handles the mouse wheel.
	scroll(notches int)
	// modal reports whether a click next to the dialog leaves it open.
	modal() bool
}

// rect is an area of the screen.
type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// open shows a dialog.
func (m *Model) open(o overlay) {
	m.overlays = append(m.overlays, o)
}

// dismiss removes a dialog, wherever it sits in the stack.
func (m *Model) dismiss(o overlay) {
	for i := range m.overlays {
		if m.overlays[i] == o {
			m.overlays = append(m.overlays[:i], m.overlays[i+1:]...)

			break
		}
	}

	if c, ok := o.(interface{ closed() }); ok {
		c.closed()
	}
}

// handleKey routes a key press: to the dialog on top, else to the input line, else to the
// page, and what the page does not use falls through to the global keys.
// normalizeKey makes a key press read the same whatever way the terminal reports it.
// With extended keyboard reporting some terminals send a shifted letter as the letter
// plus the shift modifier, without the text it types: the upper case letter is filled in,
// so that the keys of the UI are matched in one way only.
func normalizeKey(msg tea.KeyPressMsg) tea.KeyPressMsg {
	if msg.Text == "" && msg.Mod == tea.ModShift && unicode.IsLetter(msg.Code) {
		msg.Text = string(unicode.ToUpper(msg.Code))
	}

	return msg
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	msg = normalizeKey(msg)

	if m.toast.welcome {
		m.toast = toast{}
	}

	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "ctrl+z":
		// Hand the terminal back to the shell, as terminal programs do. The UI
		// carries on when it is brought to the foreground again.
		return tea.Suspend
	case "ctrl+l":
		return tea.ClearScreen
	}

	if n := len(m.overlays); n > 0 {
		o := m.overlays[n-1]

		closed, cmd := o.key(m, msg)
		if closed {
			m.dismiss(o)
		}

		return cmd
	}

	if m.prompt != nil {
		return m.prompt.key(m, msg)
	}

	// In lists the arrows walk the hierarchy: right goes into what is selected, as
	// enter does, and left goes back out.
	if m.top().table() != nil {
		switch msg.String() {
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "left":
			m.back()

			return nil
		}
	}

	if used, cmd := m.top().key(m, msg); used {
		return cmd
	}

	if a, ok := m.top().(actor); ok {
		if msg.String() == keyActions {
			return m.openActionMenu()
		}

		_, list := a.actions(m)
		if used, cmd := m.runAction(list, msg.String()); used {
			return cmd
		}
	}

	return m.globalKey(msg)
}

func (m *Model) globalKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()

	for _, t := range tabs {
		if key == t.key {
			m.goTo(t.id)

			return nil
		}
	}

	switch key {
	case "0":
		m.setScope("")
	case "[":
		return m.sibling(-1)
	case "]":
		return m.sibling(1)
	case "ctrl+k", "ctrl+p":
		return m.goToMenu()
	case ":":
		return m.openPrompt(promptCommand)
	case "/":
		if m.top().table() != nil {
			return m.openPrompt(promptFilter)
		}

		if _, ok := m.top().(finder); ok {
			return m.openPrompt(promptFind)
		}
	case "?":
		m.push(newHelpPage(m))
	case keyEsc, "left":
		m.back()
	case "q":
		if !m.back() {
			return m.quit()
		}
	case "ctrl+r":
		m.be.Refresh()
		m.notifyf(clabinternaltuitheme.LevelInfo, "refreshing")
	}

	return nil
}

// quit ends the UI. Operations that are still running would be cut short by it, so the
// user is asked first when there are any.
func (m *Model) quit() tea.Cmd {
	var running []string

	for i := range m.jobs {
		if !m.jobs[i].State.Done() {
			running = append(running, m.jobs[i].Title)
		}
	}

	if len(running) == 0 || m.quitting {
		return tea.Quit
	}

	// A second ctrl+c while the question is open quits without an answer.
	m.quitting = true

	form := newForm("Quit", "Quit", func(formValues) tea.Cmd { return tea.Quit }).
		dangerous().
		text(
			fmt.Sprintf("%d operation(s) are still running and will be cancelled:", len(running)),
			targetList(running),
		)
	form.onClose = func() { m.quitting = false }

	return m.ask(form)
}

// back undoes the most recent step into the UI: it clears the filter, then the marks,
// then closes the page, and finally leaves the view for the list of labs. It reports
// false when there is nothing left to go back from.
func (m *Model) back() bool {
	p := m.top()

	if t := p.table(); t != nil {
		if t.Filter() != "" {
			t.SetFilter("")

			return true
		}

		if t.MarkCount() > 0 {
			t.ClearMarks()

			return true
		}
	}

	if m.pop() {
		// Back on the list of labs, the lab that was entered is left.
		if m.top().tab() == tabLabs && m.scope != "" {
			m.setScope("")
		}

		return true
	}

	if p.tab() != tabLabs {
		m.goTo(tabLabs)

		return true
	}

	return false
}

func (m *Model) handleWheel(msg tea.MouseWheelMsg) {
	var notches int

	mouse := msg.Mouse()
	sideways := mouse.Mod.Contains(tea.ModShift)

	switch mouse.Button {
	case tea.MouseWheelUp:
		notches = -1
	case tea.MouseWheelDown:
		notches = 1
	case tea.MouseWheelLeft:
		notches, sideways = -1, true
	case tea.MouseWheelRight:
		notches, sideways = 1, true
	default:
		return
	}

	// With control held the wheel scales what a page draws, around the pointer.
	if z, ok := m.top().(zoomer); ok && mouse.Mod.Contains(tea.ModCtrl) && len(m.overlays) == 0 {
		if x, y := m.bodyPointer(); z.zoomAt(m, notches, x, y) {
			return
		}
	}

	// A wheel that tilts, or one turned with shift held, scrolls sideways where the
	// page has something to show there.
	if h, ok := m.top().(sideScroller); ok && sideways && len(m.overlays) == 0 {
		h.scrollSideways(notches)

		return
	}

	if n := len(m.overlays); n > 0 {
		m.overlays[n-1].scroll(notches)

		return
	}

	if s, ok := m.top().(scroller); ok {
		s.scroll(notches)

		return
	}

	if t := m.top().table(); t != nil {
		t.Scroll(notches)
	}
}

// scroller is implemented by pages that scroll something other than a table.
type scroller interface {
	scroll(notches int)
}

// sideScroller is implemented by pages that also scroll sideways.
type sideScroller interface {
	scrollSideways(notches int)
}

// clicker is implemented by pages that place their table below other content, or that
// handle clicks themselves.
type clicker interface {
	// click handles a click at a position in the body of the page. double is set for
	// the second of two quick clicks on the same spot.
	click(m *Model, x, y int, double bool)
}

// dragger is implemented by pages whose content can be moved with the mouse.
type dragger interface {
	// dragBy moves the content by the distance the pointer traveled.
	dragBy(dx, dy int)
}

// grabber is implemented by pages that let what they show be picked up and moved with
// the pointer. Positions are in cells of the body and need not be whole: a terminal that
// reports the pointer in pixels tells where in a cell it is.
type grabber interface {
	// grab is told where the button went down and reports whether the page holds on
	// to something there. It then gets every move of the pointer until the button is
	// let go, instead of the plain drag of dragger.
	grab(m *Model, x, y float64) bool
	// dragTo is told where the pointer went with the button held. free is set while
	// the user holds the key that turns off whatever helps to align things. It
	// reports whether something is being moved: a pointer that shakes a little
	// between press and release moves nothing.
	dragTo(m *Model, x, y float64, free bool) bool
	// drop is told that the button was let go.
	drop(m *Model) tea.Cmd
}

// zoomer is implemented by pages whose content is scaled with the wheel while control
// is held. It is told where the pointer is, in cells of the body, and reports whether
// it did something.
type zoomer interface {
	zoomAt(m *Model, notches int, x, y float64) bool
}

// bodyPointer returns where the pointer is in the body of the page, in cells.
func (m *Model) bodyPointer() (x, y float64) {
	return m.pointer[0] - float64(m.bodyLeft()), m.pointer[1] - float64(m.bodyLine())
}

// pointed takes note of where a message of the mouse says the pointer is. A terminal
// that was asked to reports the position in pixels: the message is handed on with the
// cell the pointer is in, which is what most of the UI goes by.
func (m *Model) pointed(msg tea.Msg) tea.Msg {
	var mouse tea.Mouse

	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouse = tea.Mouse(msg)
	case tea.MouseReleaseMsg:
		mouse = tea.Mouse(msg)
	case tea.MouseWheelMsg:
		mouse = tea.Mouse(msg)
	case tea.MouseMotionMsg:
		mouse = tea.Mouse(msg)
	default:
		return msg
	}

	m.pointerMod = mouse.Mod

	if x, y, ok := m.pixelPointer(mouse.X, mouse.Y); ok {
		m.pointer = [2]float64{x, y}
		mouse.X = max(0, min(int(x), m.width-1))
		mouse.Y = max(0, min(int(y), m.height-1))
	} else {
		// The middle of the cell.
		m.pointer = [2]float64{float64(mouse.X) + 0.5, float64(mouse.Y) + 0.5}

		return msg
	}

	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(mouse)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(mouse)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(mouse)
	default:
		return tea.MouseMotionMsg(mouse)
	}
}

// handleRelease ends a drag: what a page held on to is dropped.
func (m *Model) handleRelease() tea.Cmd {
	m.drag = nil

	g := m.grabbed
	if g == nil {
		return nil
	}

	m.grabbed = nil

	return g.drop(m)
}

// hintZone is a clickable hint in the line of hints.
type hintZone struct {
	from, to int
	key      tea.KeyPressMsg
}

// hintKey returns the key press a hint stands for. Hints that name several keys at once,
// like the arrows, have none.
func hintKey(h hint) (tea.KeyPressMsg, bool) {
	switch h.key {
	case "⏎":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, true
	case "␣":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, true
	case keyEsc:
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case keyTab:
		return tea.KeyPressMsg{Code: tea.KeyTab}, true
	}

	runes := []rune(h.key)
	if len(runes) != 1 {
		return tea.KeyPressMsg{}, false
	}

	return tea.KeyPressMsg{Code: runes[0], Text: h.key}, true
}

// handleDrag moves the content of the page with the pointer while the button is held.
func (m *Model) handleDrag(msg tea.MouseMotionMsg) {
	mouse := msg.Mouse()
	if m.drag == nil || mouse.Button != tea.MouseLeft || len(m.overlays) > 0 {
		return
	}

	if m.grabbed != nil {
		x, y := m.bodyPointer()
		free := mouse.Mod.Contains(tea.ModShift) || mouse.Mod.Contains(tea.ModAlt)

		if m.grabbed.dragTo(m, x, y, free) {
			// A press that moved something is not the first half of a double click.
			m.lastClick = time.Time{}
		}

		m.drag = &[2]int{mouse.X, mouse.Y}

		return
	}

	if d, ok := m.top().(dragger); ok {
		d.dragBy(mouse.X-m.drag[0], mouse.Y-m.drag[1])
	}

	m.drag = &[2]int{mouse.X, mouse.Y}
}

// clickOverlay hands a click to the dialog on top. A click next to it closes a dialog
// that holds nothing worth keeping.
func (m *Model) clickOverlay(mouse tea.Mouse) tea.Cmd {
	n := len(m.overlays)
	if mouse.Button != tea.MouseLeft || n > len(m.overlayRects) {
		return nil
	}

	o, area := m.overlays[n-1], m.overlayRects[n-1]

	if !area.contains(mouse.X, mouse.Y) {
		if !o.modal() {
			m.dismiss(o)
		}

		return nil
	}

	closed, cmd := o.click(m, mouse.X-area.x, mouse.Y-area.y)
	if closed {
		m.dismiss(o)
	}

	return cmd
}

func (m *Model) handleClick(msg tea.MouseClickMsg) tea.Cmd {
	mouse := msg.Mouse()

	if len(m.overlays) > 0 {
		return m.clickOverlay(mouse)
	}

	if m.prompt != nil {
		return nil
	}

	if mouse.Button == tea.MouseRight {
		return m.contextMenu(mouse)
	}

	if mouse.Button != tea.MouseLeft {
		return nil
	}

	if m.px != nil {
		for _, z := range m.px.zones {
			if z.contains(mouse.X, mouse.Y) {
				return m.handleKey(z.key)
			}
		}
	}

	if mouse.Y == 0 || (m.graphical() && mouse.Y < pixelCrumbLine) {
		for _, z := range m.tabZones {
			if mouse.X >= z.from && mouse.X < z.to {
				m.goTo(z.tab)

				break
			}
		}

		return nil
	}

	// A click on a hint does what its key does.
	if mouse.Y == m.height-2 { // the line of hints is the second to last
		for _, z := range m.hintZones {
			if mouse.X >= z.from && mouse.X < z.to {
				return m.handleKey(z.key)
			}
		}

		return nil
	}

	y := mouse.Y - m.bodyLine()
	if y < 0 || y >= m.bodyHeight() {
		return nil
	}

	// The second of two quick clicks on the same spot is a double click.
	now := m.now()
	double := m.lastClickAt == [2]int{mouse.X, mouse.Y} && now.Sub(m.lastClick) < doubleClick
	m.lastClick, m.lastClickAt = now, [2]int{mouse.X, mouse.Y}

	if double {
		m.lastClick = time.Time{}
	}

	m.drag = &[2]int{mouse.X, mouse.Y}
	x := mouse.X - m.bodyLeft()

	if c, ok := m.top().(clicker); ok {
		c.click(m, x, y, double)

		// A double click opened something: there is nothing left to hold on to.
		if g, ok := m.top().(grabber); ok && !double {
			if fx, fy := m.bodyPointer(); g.grab(m, fx, fy) {
				m.grabbed = g
			}
		}

		return nil
	}

	t := m.top().table()
	if t == nil || !t.Click(x, y) || y == 0 {
		return nil
	}

	// A double click on a row opens it, like enter does.
	if double {
		_, cmd := m.top().key(m, tea.KeyPressMsg{Code: tea.KeyEnter})

		return cmd
	}

	return nil
}

// contextMenu opens the action menu for what was clicked with the right button: the row
// under the pointer becomes the selection first.
func (m *Model) contextMenu(mouse tea.Mouse) tea.Cmd {
	y := mouse.Y - m.bodyLine()
	if y < 0 || y >= m.bodyHeight() {
		return nil
	}

	if c, ok := m.top().(clicker); ok {
		c.click(m, mouse.X-m.bodyLeft(), y, false)
	} else if t := m.top().table(); t != nil && y > 0 {
		t.Click(mouse.X-m.bodyLeft(), y)
	}

	return m.openActionMenu()
}

// openActionMenu lists what can be done with the selection of the current page.
func (m *Model) openActionMenu() tea.Cmd {
	a, ok := m.top().(actor)
	if !ok {
		return nil
	}

	title, list := a.actions(m)
	if len(list) == 0 {
		return nil
	}

	return m.actionMenu(title, list)
}

func (m *Model) handlePaste(msg tea.PasteMsg) tea.Cmd {
	if n := len(m.overlays); n > 0 {
		m.overlays[n-1].paste(msg.Content)

		return nil
	}

	if m.prompt != nil {
		return m.prompt.paste(m, msg.Content)
	}

	return nil
}

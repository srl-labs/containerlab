// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

const (
	// chromeLines is the number of lines around the body: the header, the title rule,
	// the hints and the status line.
	chromeLines = 4
	// Below this size the window cannot hold a usable screen.
	minWidth  = 60
	minHeight = 12

	// flushDelay collects the changes of a burst into one redraw.
	flushDelay = 40 * time.Millisecond

	windowTitle = "containerlab"

	// welcome is shown when the UI starts, until the first key press.
	welcome = "↑↓ move · ⏎ open · esc back · a what can I do here · ? all keys · q quit"
	// welcomeTime is how long the welcome stays when no key is pressed.
	welcomeTime = time.Minute
)

type (
	// backendMsg tells that the backend has changes to pick up.
	backendMsg struct{}
	// flushMsg tells to apply the changes collected since the backend woke the UI.
	flushMsg struct{}
	// tickMsg arrives once a second, to keep durations and the status line current.
	tickMsg time.Time
)

// Model is the root of the UI: it owns the page stack, the dialogs and the chrome around
// them, and routes the input.
type Model struct {
	be   clabinternaltuibackend.Backend
	opts Options
	th   *clabinternaltuitheme.Theme
	// dark is what the terminal said about its background, which the automatic theme
	// follows. It is assumed to be dark until the terminal answers.
	dark bool

	width  int
	height int

	host clabinternaltuibackend.Host
	labs []clabinternaltuibackend.Lab
	jobs []clabinternaltuibackend.JobInfo

	// scope is the lab the views are limited to, empty for all labs.
	scope string
	stack []page
	// roots keeps the first page of every view, so that a view is found the way it
	// was left: with its cursor, its filter and its sort order.
	roots map[tabID]page

	prompt   *prompt
	overlays []overlay
	toast    toast
	// tabZones are the clickable tabs of the header as last drawn, hintZones the
	// clickable hints, and overlayRects the areas the dialogs take.
	tabZones     []zone
	hintZones    []hintZone
	overlayRects []rect

	// lastClick and lastClickAt tell a double click, drag where the pointer was when
	// it last moved with the button held.
	lastClickAt [2]int
	drag        *[2]int

	// watched are the jobs whose end is announced in the status line.
	watched map[clabinternaltuibackend.JobID]bool
	// lastCommand is the command last run in nodes, offered again the next time.
	lastCommand string
	// lastClick is when a table row was last clicked, to tell double clicks.
	lastClick time.Time
	// quitting is set while the user is asked whether to quit with operations running.
	quitting bool

	flushPending bool

	// px is the pixel layer, nil when the UI was started without one.
	px *pixels
}

var _ tea.Model = (*Model)(nil)

// New creates the UI on top of a backend.
func New(be clabinternaltuibackend.Backend, opts Options) *Model {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	// Run rejects a theme it does not know. A model created without it falls back to the
	// default one.
	th, err := clabinternaltuitheme.New(opts.Theme, true)
	if err != nil {
		th = clabinternaltuitheme.Default(true)
	}

	m := &Model{
		be: be, opts: opts, th: th, dark: true, scope: opts.Lab,
		watched: map[clabinternaltuibackend.JobID]bool{},
		roots:   map[tabID]page{},
	}

	if opts.pixels != nil {
		m.px = newPixels(opts.pixels, opts.screen, opts.Graphics)
	}

	m.load()

	start := tabLabs
	if m.scope != "" {
		start = tabNodes
	}

	m.stack = []page{m.root(start)}
	m.be.SetFocus(m.top().focus(m))

	// What a first-time user needs to know fits one line: everything else can be
	// found from there.
	m.toast = toast{
		level:   clabinternaltuitheme.LevelInfo,
		text:    welcome,
		until:   m.now().Add(welcomeTime),
		welcome: true,
	}

	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(waitBackend(m.be), tea.RequestBackgroundColor, tick(), m.pixelInit())
}

func waitBackend(be clabinternaltuibackend.Backend) tea.Cmd {
	return func() tea.Msg {
		<-be.Notify()

		return backendMsg{}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// load reads the shared state from the backend.
func (m *Model) load() {
	m.host = m.be.Host()
	m.labs = m.be.Labs()
	m.jobs = m.be.Jobs()
}

func (m *Model) now() time.Time { return m.opts.Now() }

func (m *Model) top() page { return m.stack[len(m.stack)-1] }

// lab returns the lab with the given name from the current snapshot.
func (m *Model) lab(name string) *clabinternaltuibackend.Lab {
	for i := range m.labs {
		if m.labs[i].Name == name {
			return &m.labs[i]
		}
	}

	return nil
}

// scopedLabs returns the labs the views show: the scoped one, or all of them.
func (m *Model) scopedLabs() []clabinternaltuibackend.Lab {
	if m.scope == "" {
		return m.labs
	}

	if lab := m.lab(m.scope); lab != nil {
		return []clabinternaltuibackend.Lab{*lab}
	}

	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()

		if m.px != nil {
			// The cells may have another size now, ie. after zooming.
			return m, tea.Raw(pixelSizeQuery)
		}

		return m, nil
	case tea.BackgroundColorMsg:
		m.setBackground(clabinternaltuigfx.RGB(msg.Color))
		m.setDark(msg.IsDark())

		return m, nil
	case backendMsg:
		return m, m.onBackendWake()
	case flushMsg:
		m.flush()

		return m, nil
	case tickMsg:
		m.toast.expire(m.now())

		return m, tick()
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseWheelMsg:
		m.handleWheel(msg)

		return m, nil
	case tea.MouseClickMsg:
		return m, m.handleClick(msg)
	case tea.MouseMotionMsg:
		m.handleDrag(msg)

		return m, nil
	case tea.MouseReleaseMsg:
		m.drag = nil

		return m, nil
	case tea.PasteMsg:
		return m, m.handlePaste(msg)
	case sessionEndedMsg:
		m.sessionEnded(msg)

		return m, nil
	case describeMsg:
		m.described(msg)

		return m, nil
	}

	if m.pixelUpdate(msg) {
		return m, nil
	}

	return m, m.broadcast(msg)
}

// broadcast hands a message to every page and dialog. Results of background work, such as
// log lines, find their owner this way even when it is not the one on top.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	for _, p := range m.stack {
		if cmd := p.update(m, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	for _, o := range m.overlays {
		if cmd := o.update(m, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if m.prompt != nil {
		if cmd := m.prompt.update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// onBackendWake schedules a flush shortly after the first change of a burst and goes back
// to waiting for the backend.
func (m *Model) onBackendWake() tea.Cmd {
	cmds := []tea.Cmd{waitBackend(m.be)}

	if !m.flushPending {
		m.flushPending = true

		cmds = append(cmds, tea.Tick(flushDelay, func(time.Time) tea.Msg { return flushMsg{} }))
	}

	return tea.Batch(cmds...)
}

// flush applies what changed in the backend to the pages.
func (m *Model) flush() {
	m.flushPending = false

	topics := m.be.Changed()
	if topics == 0 {
		return
	}

	m.load()

	if m.scope != "" && m.lab(m.scope) == nil {
		m.scope = ""
	}

	for _, p := range m.stack {
		p.refresh(m, topics)
	}

	if topics.Has(clabinternaltuibackend.TopicJobs) {
		m.announceJobs()
	}
}

// announceJobs tells in the status line how the watched jobs ended, unless the user is
// looking at the output of the job anyway.
func (m *Model) announceJobs() {
	for i := range m.jobs {
		job := &m.jobs[i]
		if !m.watched[job.ID] || !job.State.Done() {
			continue
		}

		delete(m.watched, job.ID)

		switch p := m.top().(type) {
		case *jobPage:
			if p.id == job.ID {
				continue
			}
		case *capturePage:
			if p.id == job.ID {
				continue
			}
		}

		took := clabinternaltuiui.Duration(job.Duration(m.now()))

		switch job.State {
		case clabinternaltuibackend.JobSucceeded:
			m.notifyf(clabinternaltuitheme.LevelSuccess, "%s done (%s)", job.Title, took)
		case clabinternaltuibackend.JobFailed:
			m.notifyf(clabinternaltuitheme.LevelError,
				"%s failed: %s — see the jobs view (6)",
				job.Title, job.Err)
		case clabinternaltuibackend.JobCancelled:
			m.notifyf(clabinternaltuitheme.LevelWarning, "%s cancelled", job.Title)
		case clabinternaltuibackend.JobRunning, clabinternaltuibackend.JobCancelling:
		}
	}
}

func (m *Model) setDark(dark bool) {
	m.dark = dark

	if !clabinternaltuitheme.IsAuto(m.opts.Theme) {
		return
	}

	m.setTheme(clabinternaltuitheme.Default(dark))
}

func (m *Model) setTheme(th *clabinternaltuitheme.Theme) {
	m.th = th

	pages := append([]page(nil), m.stack...)
	for _, p := range m.roots {
		pages = append(pages, p)
	}

	for _, p := range pages {
		if t := p.table(); t != nil {
			t.SetTheme(th)
		}

		if themed, ok := p.(interface {
			setTheme(th *clabinternaltuitheme.Theme)
		}); ok {
			themed.setTheme(th)
		}

		p.refresh(m, clabinternaltuibackend.TopicAll)
	}
}

func (m *Model) bodyHeight() int {
	if m.graphical() {
		return max(0, m.height-pixelChrome)
	}

	return max(0, m.height-chromeLines)
}

// bodyLine is the screen line the body starts at.
func (m *Model) bodyLine() int {
	if m.graphical() {
		return pixelBodyTop
	}

	return bodyTop
}

// bodyLeft is the column the body starts at, and bodyWidth how wide it is. With the pixel
// layer it sits in a card, a little in from the edges of the window.
func (m *Model) bodyLeft() int {
	if m.graphical() {
		return pixelInset
	}

	return 0
}

func (m *Model) bodyWidth() int {
	return max(0, m.width-2*m.bodyLeft()) // the same on both sides
}

func (m *Model) layout() {
	for _, p := range m.stack {
		m.fit(p)
	}
}

// fit gives a page the room it has, and tells it how the window is drawn.
func (m *Model) fit(p page) {
	p.setPixels(m.graphical())

	if t := p.table(); t != nil {
		t.SetPixels(m.graphical())
	}

	p.resize(m.bodyWidth(), m.bodyHeight())
}

// push opens a page on top of the current one.
func (m *Model) push(p page) {
	m.fit(p)
	m.stack = append(m.stack, p)
	m.be.SetFocus(p.focus(m))
}

// pop closes the page on top. The last page stays.
func (m *Model) pop() bool {
	if len(m.stack) <= 1 {
		return false
	}

	m.top().close()
	m.stack = m.stack[:len(m.stack)-1]
	m.top().refresh(m, clabinternaltuibackend.TopicAll)
	m.be.SetFocus(m.top().focus(m))

	return true
}

// goTo shows a view from its first page. The pages opened on top of the current view
// are closed; the first page of each view is kept, so that coming back finds it as it
// was left.
func (m *Model) goTo(id tabID) {
	for _, p := range m.stack {
		if m.roots[p.tab()] != p {
			p.close()
		}
	}

	// The list of labs is the top of the hierarchy: it shows every lab, and going
	// there leaves the lab the other views were limited to.
	if id == tabLabs {
		m.scope = ""
	}

	root := m.root(id)
	m.fit(root)
	root.refresh(m, clabinternaltuibackend.TopicAll)

	m.stack = append(m.stack[:0], root)
	m.be.SetFocus(root.focus(m))
}

// root returns the first page of a view, creating it when it is shown for the first time.
func (m *Model) root(id tabID) page {
	if p, ok := m.roots[id]; ok {
		return p
	}

	p := m.rootPage(id)
	m.roots[id] = p

	return p
}

func (m *Model) rootPage(id tabID) page {
	switch id {
	case tabNodes:
		return newNodesPage(m)
	case tabInterfaces:
		return newInterfacesPage(m)
	case tabLinks:
		return newLinksPage(m)
	case tabMap:
		return newMapPage(m)
	case tabJobs:
		return newJobsPage(m)
	case tabLabs:
		return newLabsPage(m)
	default:
		return newLabsPage(m)
	}
}

// setScope limits the views to a lab, or shows all labs when the name is empty.
func (m *Model) setScope(lab string) {
	m.scope = lab

	for _, p := range m.stack {
		p.refresh(m, clabinternaltuibackend.TopicAll)
	}

	m.be.SetFocus(m.top().focus(m))
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = windowTitle

	if !m.opts.NoMouse {
		v.MouseMode = tea.MouseModeCellMotion
	}

	// The pixel layer is put back when the terminal comes into view again.
	v.ReportFocus = m.px != nil

	return v
}

// render draws the whole window as plain lines of styled text.
func (m *Model) render() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.graphical() {
		return m.renderPixels()
	}

	m.hidePixels()

	if m.width < minWidth || m.height < minHeight {
		return clabinternaltuiui.FitLines(
			m.th.Muted.Render("terminal too small"), m.width, m.height)
	}

	body := clabinternaltuiui.FitLines(m.top().view(m), m.width, m.bodyHeight())

	screen := strings.Join([]string{
		m.header(),
		m.titleRule(),
		body,
		m.hintLine(),
		m.statusLine(),
	}, "\n")

	if len(m.overlays) == 0 {
		return screen
	}

	return m.compose(screen)
}

// compose draws the dialogs centered on top of the screen.
func (m *Model) compose(screen string) string {
	layers := make([]*lipgloss.Layer, 0, len(m.overlays)+1)
	layers = append(layers, lipgloss.NewLayer(screen))

	m.overlayRects = m.overlayRects[:0]

	if sc := m.scene(); sc != nil {
		// Dim the whole window; each dialog then clears its own area again.
		sc.On(clabinternaltuigfx.Over).
			Rect(0, 0, float64(m.width), float64(m.height), m.skin().Scrim)
		sc.On(clabinternaltuigfx.Under)
	}

	for i, o := range m.overlays {
		box := o.view(m, m.width-4, m.height-2) //nolint:mnd // keep a margin around dialogs
		w, h := lipgloss.Size(box)
		x, y := max(0, (m.width-w)/2), max(0, (m.height-h)/2) //nolint:mnd // centered

		m.overlayRects = append(m.overlayRects, rect{x: x, y: y, w: w, h: h})

		if m.scene() != nil {
			box = m.pixelDialog(box, x, y, w, h)
		}

		layers = append(layers, lipgloss.NewLayer(box).X(x).Y(y).Z(i+1))
	}

	// The compositor drops trailing blanks: put them back, so that the screen keeps
	// covering the whole window.
	return clabinternaltuiui.FitLines(lipgloss.NewCompositor(layers...).Render(), m.width, m.height)
}

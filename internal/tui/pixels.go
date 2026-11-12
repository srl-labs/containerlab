// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"bytes"
	_ "embed" // the logo
	"fmt"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// Values of the graphics option.
const (
	// GraphicsAuto draws the pixel layer when the terminal says it can show it.
	GraphicsAuto = "auto"
	// GraphicsOn draws it without asking the terminal.
	GraphicsOn = "on"
	// GraphicsOff never draws it.
	GraphicsOff = "off"
)

// GraphicsModes lists the values of the graphics option.
func GraphicsModes() []string { return []string{GraphicsAuto, GraphicsOn, GraphicsOff} }

const (
	// graphicsScaleEnv sets how many pixels the layer is drawn with for each pixel the
	// terminal reports. Some terminals report sizes without the scale of the display,
	// so on a high resolution display the layer is only sharp when drawn larger.
	// Without it the scale follows from what is known about the terminal.
	graphicsScaleEnv = "CLAB_TUI_GRAPHICS_SCALE"
	// graphicsTextEnv set to off leaves the text to the terminal.
	graphicsTextEnv = "CLAB_TUI_GRAPHICS_TEXT"
	// graphicsMouseEnv tells how the terminal reports the pointer, for a terminal that
	// gets it wrong when asked: cells, or pixels.
	graphicsMouseEnv = "CLAB_TUI_GRAPHICS_MOUSE"
	mouseInCells     = "cells"
	mouseInPixels    = "pixels"
	defaultScale     = 2
	maxScale         = 4
	// pixelBudget is the most pixels the layer is drawn with, 32 MB of image memory,
	// and minScale how far the scale is taken down to stay within it.
	pixelBudget = 8_000_000
	minScale    = 0.5

	// graphicsQuery asks the terminal whether it shows images of the kitty graphics
	// protocol, with an image of one pixel that is not kept.
	graphicsQuery = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\"
	// versionQuery asks the terminal for its name and version, and flatTerminal is
	// the name of the terminals that show one image a cell, on top of the text.
	versionQuery = "\x1b[>0q"
	flatTerminal = "xterm.js"
	// pixelSizeQuery asks for the size of the text area in pixels.
	pixelSizeQuery = "\x1b[14t"
	// sharpCell is the height of a cell in pixels from which the layer is drawn with
	// the pixels the terminal reports: such cells are those of a display of high
	// resolution, counted in its own pixels.
	sharpCell = 30
	// pixelMouseQuery asks whether the terminal knows how to report the pointer in
	// pixels, and pixelMouseTerminal is the name of the terminals that do without
	// saying so: the ones built on the par-term engine, Muxus among them.
	pixelMouseQuery    = "\x1b[?1016$p"
	pixelMouseTerminal = "par-term"
	// pixelAck is asked after the pixels of a frame, and answered once the terminal
	// took them in: the next frame is not drawn before, so that a terminal that is
	// slow with images shows the latest state rather than one frame after the other
	// of what happened a while ago. It is the question about the pointer again, as
	// that is one every terminal that knows such questions answers.
	pixelAck = pixelMouseQuery
	// ackTimeout is how long the answer is waited for, and ackMisses how many times
	// in a row it may stay away before it is no longer asked for.
	ackTimeout = 1500 * time.Millisecond
	ackMisses  = 3
	// graphicsTraceEnv names a file that is told what every frame of the pixel layer
	// took: to find out why a terminal is slow.
	graphicsTraceEnv = "CLAB_TUI_GRAPHICS_TRACE"
	// pixelMouseSlack is how far beyond the window, as a multiple of its size, a
	// position may be before the terminal is taken to report in another unit than
	// the one it gives the size of the window in.
	pixelMouseSlack = 1.5

	// The pixel layer needs some room: below this size the plain look is used.
	pixelMinWidth  = 72
	pixelMinHeight = 20

	// The lines of the window with a pixel layer: three for the header, one for the
	// path, then the body in a card that takes a line above and below, the hints and
	// the status line.
	pixelCrumbLine = 3
	pixelBodyTop   = 5
	pixelChrome    = pixelBodyTop + 3
	// pixelInset is where the content of a card starts, from the edge of the window.
	pixelInset = 2
)

// deviceTerminals are the names of the terminals that report sizes in the pixels of the
// display, not in pixels that the display scales.
var deviceTerminals = []string{"ghostty", "kitty", "wezterm", "foot"}

// pixels is the state of the pixel layer drawn under the text, for terminals that speak
// the kitty graphics protocol.
type pixels struct {
	out    *clabinternaltuigfx.Output
	screen *clabinternaltuigfx.Screen
	// supported is set once the terminal confirmed that it shows images.
	supported bool
	scale     int
	// w and h are the size of the text area in pixels as the terminal last reported
	// it, cols and rows the size in cells it had then.
	w, h       int
	cols, rows int
	// bg is the background color of the terminal, when it told.
	bg    clabinternaltuigfx.Color
	hasBG bool
	skin  clabinternaltuiui.Skin
	// shown is set while the layer is on screen.
	shown bool
	// text is set when the layer writes the text as well, in a font of its own.
	text bool
	// devicePixels is set for a terminal that is known to report sizes in the pixels
	// of the display, scaledPixels for one known to report them without the scale
	// of the display.
	devicePixels, scaledPixels bool
	// mouseCan is set once the terminal is known to report the pointer in pixels when
	// asked to, mouseOn while it is asked to, and mouseOff when it never is.
	mouseCan, mouseOn, mouseOff bool
	// acks is set once the terminal answered a question about a mode, which is what
	// tells that it took a frame in. inFlight is set while the answer for the frame
	// sent at sentAt is awaited, and missed counts the answers that never came.
	// late counts the answers that may still come for frames that were given up on.
	acks, inFlight bool
	sentAt         time.Time
	missed, late   int
	// trace is the file the frames are logged to, nil for none.
	trace *os.File
	// flat is set for a terminal that shows one image a cell, on top of its text, and
	// nothing below it: the terminal of VS Code and whatever else is built on
	// xterm.js. The layer is then sent as one image a row and writes all of the
	// text, also what a node printed, which it sets in a font of fixed width.
	flat bool

	// scene is the frame being drawn, drawn the one drawn last, and zones what can be
	// clicked in it.
	scene *clabinternaltuigfx.Scene
	drawn *clabinternaltuigfx.Scene
	zones []pixelZone
}

// tablePlacer is implemented by the pages whose table does not start at the top of the
// body. It tells the line it starts at.
type tablePlacer interface{ tableLine() int }

// monospaced is implemented by the pages whose body is literal text, which the terminal
// keeps drawing in its fixed width font.
type monospaced interface{ monospaced() bool }

// pixelOverlays draws the dialogs with the pixel layer, text included: each on its card,
// over a window that is dimmed.
func (m *Model) pixelOverlays(sc *clabinternaltuigfx.Scene) {
	m.overlayRects = m.overlayRects[:0]

	sc.On(clabinternaltuigfx.Over).
		Rect(0, 0, float64(m.width), float64(m.height), m.skin().Scrim)
	sc.On(clabinternaltuigfx.Under)

	for _, o := range m.overlays {
		box := o.view(m, m.width-4, m.height-2) //nolint:mnd // keep a margin around dialogs
		w, h := lipgloss.Size(box)
		x, y := max(0, (m.width-w)/2), max(0, (m.height-h)/2) //nolint:mnd // centered

		m.overlayRects = append(m.overlayRects, rect{x: x, y: y, w: w, h: h})

		for i, line := range strings.Split(m.pixelDialog(box, x, y, w, h), "\n") {
			m.pixelLine(sc, y+i, x, line, true)
		}
	}
}

// prepared is implemented by the pages that lay out what they show with the scene of a
// frame, before the frame is drawn.
type prepared interface {
	prepare(m *Model, sc *clabinternaltuigfx.Scene)
}

// selfFramed is implemented by the pages that draw the cards of their body themselves.
type selfFramed interface{ framesItself() bool }

// pixelZone is something drawn by the pixel layer that reacts to a click like a key.
type pixelZone struct {
	rect
	key tea.KeyPressMsg
}

func newPixels(
	out *clabinternaltuigfx.Output,
	screen *clabinternaltuigfx.Screen,
	mode string,
) *pixels {
	// Zero leaves the scale to what is known about the terminal, see metrics.
	scale := 0
	if n, err := strconv.Atoi(os.Getenv(graphicsScaleEnv)); err == nil && n >= 1 && n <= maxScale {
		scale = n
	}

	var trace *os.File
	if name := os.Getenv(graphicsTraceEnv); name != "" {
		//nolint:mnd // the mode of a log file
		trace, _ = os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	}

	return &pixels{
		trace: trace,
		out:   out, screen: screen, supported: mode == GraphicsOn, scale: scale,
		text:     os.Getenv(graphicsTextEnv) != "off" && uiFont() != nil,
		flat:     os.Getenv("TERM_PROGRAM") == "vscode",
		mouseCan: os.Getenv(graphicsMouseEnv) == mouseInPixels,
		mouseOff: os.Getenv(graphicsMouseEnv) == mouseInCells,
	}
}

// pixelMouse asks the terminal to report the pointer in pixels once it is known that it
// can, and that there is a pixel layer to make use of it: things drawn there are then
// moved as finely as they are drawn.
func (m *Model) pixelMouse() {
	px := m.px
	want := px.mouseCan && !px.mouseOff && !m.opts.NoMouse && px.supported &&
		px.w > 0 && px.h > 0 && px.cols > 0 && px.rows > 0

	if want != px.mouseOn {
		px.mouseOn = want
		px.out.PixelMouse(want)
	}
}

// pixelPointer turns a position the terminal reported in pixels into cells, with the
// part of the cell the pointer is at. It reports false when positions come in cells.
func (m *Model) pixelPointer(x, y int) (col, row float64, ok bool) {
	px := m.px
	if px == nil || !px.mouseOn {
		return 0, 0, false
	}

	// A position far outside of the window is in another unit than the size of the
	// window was given in: such a terminal is left to report cells.
	if float64(x) > pixelMouseSlack*float64(px.w) || float64(y) > pixelMouseSlack*float64(px.h) {
		px.mouseOff = true

		m.pixelMouse()

		return 0, 0, false
	}

	// The input decoder took one off, as it does for cells, which count from one.
	col = float64(x+1) * float64(px.cols) / float64(px.w)
	row = float64(y+1) * float64(px.rows) / float64(px.h)

	return max(0, col), max(0, row), true
}

// cellAspect returns the height of a cell over its width.
func (m *Model) cellAspect() float64 {
	const usual = 2.25

	px := m.px
	if px == nil || px.w <= 0 || px.h <= 0 || px.cols <= 0 || px.rows <= 0 {
		return usual
	}

	return (float64(px.h) / float64(px.rows)) / (float64(px.w) / float64(px.cols))
}

// graphical reports whether the window is drawn with the pixel layer.
func (m *Model) graphical() bool {
	px := m.px

	return px != nil && px.supported && px.w > 0 && px.h > 0 && px.cols > 0 && px.rows > 0 &&
		m.width >= pixelMinWidth && m.height >= pixelMinHeight &&
		// A terminal that cannot draw under its text needs the layer to write it.
		(px.text || !px.flat)
}

// scene returns the frame of the pixel layer being drawn, nil when there is none.
func (m *Model) scene() *clabinternaltuigfx.Scene {
	if m.px == nil {
		return nil
	}

	return m.px.scene
}

func (m *Model) skin() *clabinternaltuiui.Skin { return &m.px.skin }

// pixelInit returns the questions the terminal is asked at the start.
func (m *Model) pixelInit() tea.Cmd {
	if m.px == nil {
		return nil
	}

	// The terminal is asked who it is first: the answers come in the order of the
	// questions, so it is known what kind of terminal says that it shows images.
	return tea.Raw(versionQuery + graphicsQuery + pixelSizeQuery + pixelMouseQuery)
}

// pixelUpdate handles what the terminal answers. It reports whether the message was one
// of the answers.
func (m *Model) pixelUpdate(msg tea.Msg) bool {
	if m.px == nil {
		return false
	}

	defer m.pixelMouse()

	switch msg := msg.(type) {
	case tea.TerminalVersionMsg:
		name := strings.ToLower(msg.Name)

		if strings.HasPrefix(name, flatTerminal) && !m.px.flat {
			m.px.flat = true
			m.layout()
		}

		// The terminals of that engine report sizes the way they report the pointer.
		if strings.HasPrefix(name, pixelMouseTerminal) {
			m.px.mouseCan, m.px.scaledPixels = true, true
		}

		for _, known := range deviceTerminals {
			if strings.HasPrefix(name, known) && !m.px.devicePixels {
				m.px.devicePixels = true
				m.layout()
			}
		}

		return true
	case tea.ModeReportMsg:
		if msg.Mode != ansi.ModeMouseExtSgrPixel {
			return false
		}

		// Whatever the answer, it tells that the terminal answers, and that it is
		// done with the frame the question was sent after.
		m.frameAcked()

		switch msg.Value {
		case ansi.ModeSet, ansi.ModeReset, ansi.ModePermanentlySet:
			m.px.mouseCan = true
		case ansi.ModeNotRecognized, ansi.ModePermanentlyReset:
		}

		return true
	case uv.KittyGraphicsEvent:
		if bytes.HasPrefix(msg.Payload, []byte("OK")) {
			m.px.supported = true
			m.layout()
		}

		return true
	case tea.FocusMsg:
		// A terminal may drop images of a window that is not in view, to save
		// memory: put the layer back when the window is looked at again.
		if m.graphical() && m.px.shown && !m.px.flat {
			m.px.out.Queue(nil, m.px.screen.All())
		}

		return true
	case tea.BlurMsg:
		return true
	case uv.PixelSizeEvent:
		if msg.Width > 0 && msg.Height > 0 && m.width > 0 && m.height > 0 {
			m.px.w, m.px.h, m.px.cols, m.px.rows = msg.Width, msg.Height, m.width, m.height
			m.layout()
		}

		return true
	}

	return false
}

// sendFrame hands the pixels of a frame to the terminal, with the question whose answer
// tells that the terminal is ready for the next one.
func (m *Model) sendFrame(sc *clabinternaltuigfx.Scene) {
	px := m.px
	start := time.Now()
	before, after := px.screen.Update(sc)

	if px.acks && len(after) > 0 {
		// Answers that did not come for this long are not coming anymore.
		if time.Since(px.sentAt) > 2*ackTimeout {
			px.late = 0
		}

		after = append(after, pixelAck...)
		px.inFlight, px.sentAt = true, time.Now()
	}

	px.out.Queue(before, after)

	// What follows the pointer or a key is not held back for the text.
	if m.grabbed != nil || len(after) > 0 && time.Since(m.lastInput) < time.Second {
		px.out.Hurry()
	}

	if px.trace != nil && len(after) > 0 {
		fmt.Fprintf(px.trace, "%s frame %d bytes, drawn in %.1f ms\n",
			start.Format("15:04:05.000"), len(before)+len(after),
			float64(time.Since(start).Microseconds())/1000) //nolint:mnd // to milliseconds
	}
}

// traceInput logs a key or a move of the mouse, when the frames are logged: how often
// a terminal reports the pointer bounds how smoothly something follows it.
func (m *Model) traceInput(msg tea.Msg) {
	if m.px == nil || m.px.trace == nil {
		return
	}

	fmt.Fprintf(m.px.trace, "%s input %T\n", time.Now().Format("15:04:05.000"), msg)
}

// frameAcked takes note that the terminal is done with the frame it was sent last.
func (m *Model) frameAcked() {
	px := m.px
	px.acks = true

	// An answer for a frame that was given up on says nothing about the one after.
	if px.late > 0 {
		px.late--

		return
	}

	px.missed = 0

	if !px.inFlight {
		return
	}

	px.inFlight = false

	if px.trace != nil {
		fmt.Fprintf(px.trace, "%s taken in after %.1f ms\n", time.Now().Format("15:04:05.000"),
			float64(time.Since(px.sentAt).Microseconds())/1000) //nolint:mnd // to milliseconds
	}
}

// frameLate is called when the answer for a frame is overdue: the next frame is drawn
// without it, and a terminal that keeps not answering is no longer asked.
func (m *Model) frameLate() {
	px := m.px
	if px == nil || !px.inFlight || time.Since(px.sentAt) < ackTimeout {
		return
	}

	px.inFlight = false
	px.late++

	px.missed++

	if px.missed >= ackMisses {
		px.acks, px.late = false, 0
	}
}

// holding reports whether the window is not drawn anew for now: while the terminal is
// busy with the frame before, or, in a terminal that does not tell, while something is
// dragged and the last frame is a moment old only.
func (m *Model) holding() bool {
	px := m.px
	if px == nil || m.frame == "" {
		return false
	}

	if px.inFlight {
		return time.Since(px.sentAt) < ackTimeout
	}

	return m.grabbed != nil && !px.acks && time.Since(m.frameAt) < frameGap
}

// metrics returns the size of the layer for the current window. Between a resize and the
// answer of the terminal to the question about its new size, the size of the cells is
// taken to be what it was.
//
// A terminal keeps what it holds of images within a budget, and drops the oldest ones
// when that is exceeded: the layer is drawn with fewer pixels in a large window, so that
// it stays well within what terminals allow.
func (m *Model) metrics() clabinternaltuigfx.Metrics {
	px := m.px
	width := float64(px.w) / float64(px.cols) * float64(m.width)
	height := float64(px.h) / float64(px.rows) * float64(m.height)

	scale := float64(px.scale)

	switch {
	case px.flat:
		// A terminal built on xterm.js draws images with the pixels it reports,
		// whatever the display has: more of them are scaled down again, at four
		// times the cost.
		scale = 1
	case px.scale > 0:
	case px.devicePixels || (!px.scaledPixels && height/float64(m.height) >= sharpCell):
		// The terminal reports the pixels of the display, or its cells are large
		// enough to look sharp as they are: more pixels would only be more data.
		scale = 1
	default:
		// A terminal that reports its size without the scale of the display: twice
		// the pixels keep the layer sharp on a high resolution one.
		scale = defaultScale
	}

	if most := math.Sqrt(pixelBudget / (width * height)); scale > most {
		scale = math.Max(most, minScale)
	}

	return clabinternaltuigfx.Metrics{
		Cols:   m.width,
		Rows:   m.height,
		Width:  max(m.width, int(math.Round(width*scale))),
		Height: max(m.height, int(math.Round(height*scale))),
	}
}

// setBackground takes note of the background color of the terminal, which the surfaces
// of the layer are derived from.
func (m *Model) setBackground(c clabinternaltuigfx.Color) {
	if m.px == nil {
		return
	}

	m.px.bg, m.px.hasBG = c, true
}

// buildSkin derives the colors of the layer from the theme and the terminal background.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) buildSkin() {
	px := m.px
	p := m.th.Palette

	bg := px.bg
	if !px.hasBG {
		bg = clabinternaltuigfx.Color{R: 0x0d, G: 0x11, B: 0x17, A: 0xff}
		if !m.dark {
			bg = clabinternaltuigfx.Color{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		}
	}

	// Surfaces are the background moved towards the text color: lighter on a dark
	// terminal, darker on a light one.
	ink := clabinternaltuigfx.Color{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	shade := 0.55

	if bg.Luma() > 0.5 { // a light background
		ink = clabinternaltuigfx.Color{A: 0xff}
		shade = 0.3
	}

	lift := func(t float64) clabinternaltuigfx.Color {
		return clabinternaltuigfx.Mix(bg, ink, t)
	}
	black := clabinternaltuigfx.Color{A: 0xff}

	//nolint:mnd // how far each surface is from the background
	px.skin = clabinternaltuiui.Skin{
		Font:       m.textFont(),
		Bar:        lift(0.022),
		Card:       lift(0.045),
		CardEdge:   lift(0.13),
		Line:       lift(0.09),
		RowLine:    ink.Alpha(0.04),
		Ink:        ink,
		TextSize:   textSize,
		Raised:     clabinternaltuigfx.RGB(p.Raised),
		RaisedEdge: ink.Alpha(0.09),
		Control:    clabinternaltuigfx.RGB(p.Border),
		Selection:  clabinternaltuigfx.RGB(p.Selection),
		Accent:     clabinternaltuigfx.RGB(p.Accent),
		OnAccent:   clabinternaltuigfx.RGB(p.OnAccent),
		Thumb:      lift(0.26),
		Shadow:     black.Alpha(0.5),
		Scrim:      black.Alpha(shade),
	}
}

// newScene starts the frame of the pixel layer for the kind of terminal there is.
func (m *Model) newScene() *clabinternaltuigfx.Scene {
	if m.px.flat {
		return clabinternaltuigfx.NewFlatScene(m.metrics())
	}

	return clabinternaltuigfx.NewScene(m.metrics())
}

// textFont returns the font the layer writes text with, nil when the terminal does.
func (m *Model) textFont() *clabinternaltuigfx.Font {
	if m.px == nil || !m.px.text {
		return nil
	}

	return uiFont()
}

// hidePixels takes the layer off the screen, when the window is drawn plain again.
func (m *Model) hidePixels() {
	px := m.px
	if px == nil || !px.shown {
		return
	}

	px.shown, px.scene = false, nil
	px.zones = px.zones[:0]
	px.out.Queue(px.screen.Hide(), nil)
	px.screen.Forget()
}

//go:embed assets/logo-outline.png
var logoOutlinePNG []byte

//go:embed assets/logo-fill.png
var logoFillPNG []byte

// brandColor is the blue of the containerlab logo.
//
//nolint:mnd // the components of the color
var brandColor = clabinternaltuigfx.Color{R: 0x00, G: 0xc9, B: 0xff, A: 0xff}

//go:embed assets/wordmark-text.png
var wordmarkTextPNG []byte

//go:embed assets/wordmark-lab.png
var wordmarkLabPNG []byte

func loadMask(data []byte) *clabinternaltuigfx.Mask {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	return clabinternaltuigfx.NewMask(img)
}

// logo returns the two parts of the containerlab logo: the outline of the flask, and the
// liquid in it.
var logo = sync.OnceValues(func() (outline, liquid *clabinternaltuigfx.Mask) {
	return loadMask(logoOutlinePNG), loadMask(logoFillPNG)
})

// wordmark returns the two parts of the containerlab wordmark: the letters in the color
// of the text, and the ones in the brand color.
var wordmark = sync.OnceValues(func() (text, lab *clabinternaltuigfx.Mask) {
	return loadMask(wordmarkTextPNG), loadMask(wordmarkLabPNG)
})

// textRow builds a line of text out of pieces that start at given columns.
type textRow struct {
	b strings.Builder
	w int
}

// at writes text starting at a column, or right after what is there when that column is
// taken already.
func (r *textRow) at(col int, text string) *textRow {
	if col > r.w {
		r.b.WriteString(strings.Repeat(" ", col-r.w))
		r.w = col
	}

	r.b.WriteString(text)
	r.w += clabinternaltuiui.Width(text)

	return r
}

func (r *textRow) line(width int) string {
	if r.w > width {
		return clabinternaltuiui.Truncate(r.b.String(), width)
	}

	return r.b.String() + strings.Repeat(" ", width-r.w)
}

// renderPixels draws the window with the pixel layer: the text as lines, and the scene
// that goes under it.
func (m *Model) renderPixels() string {
	px := m.px
	m.buildSkin()

	sc := m.newScene()
	px.scene, px.shown = sc, true
	px.zones = px.zones[:0]

	// A page that works out what it shows from the size of things in pixels does so
	// before anything is drawn: the line above its body tells what came of it.
	p := m.top()
	if s, ok := p.(prepared); ok {
		s.prepare(m, sc)
	}

	blank := strings.Repeat(" ", m.width)
	lines := make([]string, 0, m.height)
	lines = append(lines, blank, m.pixelHeader(sc), blank, m.pixelCrumbs(sc), blank)

	// The body sits in a card. A page made of a table gets the card of the table,
	// with its decorations; any other page gets a plain one, drawn first so that the
	// page can draw on it.
	if f, ok := p.(selfFramed); p.table() == nil && (!ok || !f.framesItself()) {
		m.card(sc, pixelBodyTop, m.bodyHeight())
	}

	body := clabinternaltuiui.FitLines(p.view(m), m.bodyWidth(), m.bodyHeight())
	lines = append(lines, indent(body, pixelInset, m.width)...)

	if t := p.table(); t != nil {
		line := 0
		if placed, ok := p.(tablePlacer); ok {
			line = placed.tableLine()
		}

		t.Paint(sc, pixelInset, float64(pixelBodyTop+line), m.skin())
	}

	lines = append(lines, blank, m.pixelHints(sc), m.pixelStatus())

	// With its own font the layer writes the text as well, and the terminal is left
	// with blank lines. The body of a page of literal text, ie. a log, stays with
	// the terminal: it wants its fixed width, and to be selected and copied.
	mono := false
	if lit, ok := p.(monospaced); ok {
		mono = lit.monospaced()
	}

	if px.text {
		for i := range lines {
			switch body := i >= pixelBodyTop && i < pixelBodyTop+m.bodyHeight(); {
			case mono && body && !px.flat:
				continue
			case mono && body:
				// Where images hide the text of the terminal, the layer writes
				// this text too, in a font of fixed width.
				m.pixelCells(sc, i, 0, lines[i])
			default:
				m.pixelLine(sc, i, 0, lines[i], false)
			}

			lines[i] = blank
		}
	}

	screen := clabinternaltuiui.StripMarks(strings.Join(lines, "\n"))

	switch {
	case len(m.overlays) == 0:
	case px.text && (!mono || px.flat):
		m.pixelOverlays(sc)
	default:
		screen = clabinternaltuiui.StripMarks(m.compose(screen))
	}

	m.sendFrame(sc)
	px.scene, px.drawn = nil, sc

	return screen
}

// Sizes of the chrome, in points and fractions of a cell.
const (
	pillRound   = 14
	chipRound   = 9
	capRound    = 4
	dialogRound = 12
	// The logo and the wordmark next to it: their heights in points, and the columns
	// they are placed at.
	logoHeight     = 27
	logoCenter     = 3.2
	wordmarkHeight = 8.2
	wordmarkCol    = 5.6
	// tabsCol is where the tabs start when the wordmark is left out, and headerGap
	// the room between the parts of the header.
	tabsCol    = 7
	headerGap  = 2
	tintAlpha  = 0.14
	edgeAlpha  = 0.45
	headerRows = 2.7
)

// tabBox is a tab of the header: where it is and what it shows.
type tabBox struct {
	x, w   int
	def    tabDef
	title  string
	active bool
}

// pixelTabs lays the tabs out from a column on.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelTabs(from int, short bool) (boxes []tabBox, end int) {
	active := m.top().tab()
	x := from

	for _, t := range tabs {
		title := t.title
		if short {
			title = t.short
		}

		if t.id == tabJobs {
			if n := m.runningJobs(); n > 0 {
				title += fmt.Sprintf("·%d", n)
			}
		}

		// The key, a space and the title, with a cell of padding on each side.
		w := clabinternaltuiui.Width(t.key) + 1 + clabinternaltuiui.Width(title) + 2
		boxes = append(boxes, tabBox{x: x, w: w, def: t, title: title, active: t.id == active})
		x += w + 1
	}

	return boxes, x - 1
}

// pixelSegment is a piece of the context shown at the right of the header.
type pixelSegment struct {
	text  string
	width int
	draw  func(sc *clabinternaltuigfx.Scene, x float64)
}

// pixelContext summarizes what the views cover, like headerContext does: the scope, how
// many nodes are ready, and how much traffic flows.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelContext(detail int) []pixelSegment {
	labs := m.scopedLabs()
	sk := m.skin()

	var ready, total int

	var rx, tx float64

	sampled := false

	for i := range labs {
		if !labs[i].Deployed {
			continue
		}

		total += len(labs[i].Nodes)
		ready += labs[i].ReadyNodes()

		for n := range labs[i].Nodes {
			rx += labs[i].Nodes[n].RxBps
			tx += labs[i].Nodes[n].TxBps
			sampled = sampled || labs[i].Nodes[n].HasStats
		}
	}

	scope := "all labs"
	if m.scope != "" {
		scope = m.scope
	}

	chip := m.th.Bold.Render(scope)
	if detail == contextFull && m.host.Runtime != "" {
		chip += m.th.Muted.Render(" · " + m.host.Runtime)
	}

	chipWidth := clabinternaltuiui.Width(chip) + 2 // a cell of padding on each side

	segs := []pixelSegment{{
		text: clabinternaltuiui.MarkCenter(" " + chip + " "), width: chipWidth,
		draw: func(sc *clabinternaltuigfx.Scene, x float64) {
			sc.Box(x-0.3, 0.8, float64(chipWidth)+0.6, 1.4, pillRound, sk.Raised, sk.RaisedEdge, 1)
		},
	}}

	level := clabinternaltuitheme.LevelSuccess

	switch {
	case total == 0:
		level = clabinternaltuitheme.LevelMuted
	case ready < total:
		level = clabinternaltuitheme.LevelWarning
	}

	count := fmt.Sprintf("%d/%d", ready, total)

	tint := clabinternaltuigfx.RGB(m.th.LevelStyle(level).GetForeground())
	share := 0.0

	if total > 0 {
		share = float64(ready) / float64(total)
	}

	segs = append(segs, pixelSegment{
		text:  "  " + m.th.Text.Render(count),
		width: 2 + clabinternaltuiui.Width(count), // room for the ring
		draw: func(sc *clabinternaltuigfx.Scene, x float64) {
			// A ring that fills with the share of nodes that are ready.
			sc.Ring(x+0.6, 1.5, 6, 2.4, tint.Alpha(0.25))

			if share > 0 {
				sc.Arc(x+0.6, 1.5, 4.8, 2.4, 0, share, tint) // inside the track
			}
		},
	})

	if detail == contextFull && sampled {
		text := m.th.Info.Render(arrowRx) + m.th.Text.Render(headerRate(rx)) + " " +
			m.th.Accent.Render(arrowTx) + m.th.Text.Render(headerRate(tx))
		segs = append(segs, pixelSegment{text: text, width: clabinternaltuiui.Width(text)})
	}

	return segs
}

func segmentsWidth(segs []pixelSegment) int {
	w := 0
	for i := range segs {
		w += segs[i].width + headerGap
	}

	return w
}

// headerLayout fits the header into the window: the tabs, the context at the right, and
// whether there is room for the wordmark. On narrow terminals the header degrades step by
// step: the wordmark goes, leaving the logo, then the titles of the tabs get shorter, then
// the context shrinks.
func (m *Model) headerLayout(
	sc *clabinternaltuigfx.Scene,
) (boxes []tabBox, segs []pixelSegment, named bool) {
	// The wordmark takes as many columns as its drawing is wide, and a gap.
	letters, _ := wordmark()
	width := sc.Pt(wordmarkHeight) * letters.Aspect() / sc.X(1)
	afterMark := int(math.Ceil(wordmarkCol+width)) + headerGap

	named = true
	short, detail := false, contextFull

	for {
		from := tabsCol
		if named {
			from = afterMark
		}

		var end int

		boxes, end = m.pixelTabs(from, short)
		segs = m.pixelContext(detail)

		if end+headerGap+segmentsWidth(segs) <= m.width || detail == contextMinimal {
			return boxes, segs, named
		}

		switch {
		case named:
			named = false
		case !short:
			short = true
		default:
			detail--
		}
	}
}

// pixelHeader draws the bar at the top: the brand, the tabs and the context.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelHeader(sc *clabinternaltuigfx.Scene) string {
	sk := m.skin()
	width := float64(m.width)

	sc.Rect(0, 0, width, headerRows, sk.Bar)
	sc.HLine(0, headerRows, width, sk.Line)
	// The logo: the outline in the color of the text, the liquid in the brand color.
	outline, liquid := logo()
	sc.Stamp(outline, logoCenter, 1.5, logoHeight, sk.Ink)
	sc.Stamp(liquid, logoCenter, 1.5, logoHeight, brandColor)

	boxes, segs, named := m.headerLayout(sc)
	if named {
		letters, lab := wordmark()
		sc.StampAt(letters, wordmarkCol, 1.5, wordmarkHeight, sk.Ink)
		sc.StampAt(lab, wordmarkCol, 1.5, wordmarkHeight, brandColor)
	}

	var row textRow

	m.tabZones = m.tabZones[:0]

	for _, b := range boxes {
		keyStyle, titleStyle := m.th.Faint, m.th.Text

		if b.active {
			sc.Box(
				float64(b.x),
				0.75,
				float64(b.w),
				1.5,
				pillRound, // around the text line
				sk.Accent.Alpha(tintAlpha),
				sk.Accent.Alpha(edgeAlpha),
				1,
			)

			keyStyle, titleStyle = m.th.Accent, m.th.Title
		}

		row.at(b.x+1, clabinternaltuiui.MarkCenter(
			keyStyle.Render(b.def.key)+" "+titleStyle.Render(b.title)))
		m.tabZones = append(m.tabZones, zone{from: b.x, to: b.x + b.w, tab: b.def.id})
	}

	x := m.width - segmentsWidth(segs)
	for i := range segs {
		if x < row.w+1 {
			break
		}

		if segs[i].draw != nil {
			segs[i].draw(sc, float64(x))
		}

		row.at(x, segs[i].text)
		x += segs[i].width + headerGap
	}

	return row.line(m.width)
}

// title returns a crumb with its first letter in upper case.
func title(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}

	return s
}

// pixelCrumbs draws the line under the header: where the user is on the left, and the
// state of the view as chips on the right.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelCrumbs(sc *clabinternaltuigfx.Scene) string {
	p := m.top()
	sk := m.skin()
	y := float64(pixelCrumbLine)

	var row textRow

	crumbs := p.crumbs(m)
	x := pixelInset

	for i, c := range crumbs {
		last := i == len(crumbs)-1

		// A count in brackets at the end of the path becomes a badge.
		badge := ""
		if open := strings.LastIndex(c, " ["); last && open > 0 && strings.HasSuffix(c, "]") {
			c, badge = c[:open], c[open+2:len(c)-1]
		}

		style := m.th.Muted
		if last {
			style = m.th.Bold
		}

		row.at(x, style.Render(c))
		x += clabinternaltuiui.Width(c) + 1

		if !last {
			row.at(x, m.th.Faint.Render("›"))
			x += 2

			continue
		}

		if badge != "" {
			w := clabinternaltuiui.Width(badge)
			sc.Fill(float64(x)+0.2, y+0.08, float64(w)+1.6, 0.84, chipRound, sk.Raised)
			row.at(x+1, clabinternaltuiui.MarkCenter(m.th.Text.Render(badge)))
		}
	}

	m.pixelInfo(sc, &row, p.info(m))

	return row.line(m.width)
}

// pixelInfo writes the state of the view as chips at the right of the line of the path.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelInfo(sc *clabinternaltuigfx.Scene, row *textRow, info []string) {
	const chip = 3 // the padding of a chip and the gap to the next one

	total := 0
	for _, seg := range info {
		total += clabinternaltuiui.Width(seg) + chip
	}

	x := m.width - pixelInset - total + 1
	y := float64(pixelCrumbLine)

	for _, seg := range info {
		w := clabinternaltuiui.Width(seg)

		if x >= row.w+chip { // keep clear of the path
			sc.Fill(
				float64(x)-0.8,
				y+0.08,
				float64(w)+1.6,
				0.84,
				chipRound, // around the text
				m.skin().Accent.Alpha(tintAlpha),
			)
			row.at(x, clabinternaltuiui.MarkCenter(m.th.Accent.Render(seg)))
		}

		x += w + chip
	}
}

// pixelHints draws the keys of the current context as key caps on a bar.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelHints(sc *clabinternaltuigfx.Scene) string {
	hints := m.top().hints(m)
	if n := len(m.overlays); n > 0 {
		hints = m.overlays[n-1].hints()
	}

	if m.prompt != nil {
		hints = m.prompt.hints()
	}

	const gap = 3

	sk := m.skin()
	y := float64(m.height - 2) // the second to last line

	sc.Rect(0, y-0.4, float64(m.width), 2.4, sk.Bar)
	sc.HLine(0, y-0.4, float64(m.width), sk.Line) // the edge of the bar

	width := func(h hint) int {
		return clabinternaltuiui.Width(h.key) + 1 + clabinternaltuiui.Width(h.desc)
	}

	// The last hint, help, always stays: the others are left out from the end when
	// the line runs out of room.
	budget := m.width - 2*pixelInset
	if len(hints) > 0 {
		budget -= width(hints[len(hints)-1]) + gap
	}

	var row textRow

	m.hintZones = m.hintZones[:0]
	x := pixelInset

	for i, h := range hints {
		last := i == len(hints)-1
		if !last && x+width(h)-pixelInset > budget {
			continue
		}

		kw := clabinternaltuiui.Width(h.key)

		// The cap is as wide as its cells, or as the key when the font draws that wider.
		capWidth := float64(kw) + 1
		if font := m.textFont(); font != nil {
			capWidth = max(capWidth, sc.TextWidth(font, textSize, true, h.key)+0.9)
		}

		sc.Box(
			float64(x)+float64(kw)/2-capWidth/2,
			y+0.05,
			capWidth,
			0.9,
			capRound,
			sk.Raised,
			sk.RaisedEdge,
			1,
		)
		row.at(
			x,
			clabinternaltuiui.MarkCenter(m.th.Bold.Render(h.key))+" "+m.th.KeyDesc.Render(h.desc),
		)

		if key, ok := hintKey(h); ok {
			m.hintZones = append(m.hintZones, hintZone{from: x - 1, to: x + width(h), key: key})
		}

		x += width(h) + gap
	}

	return row.line(m.width)
}

// pixelStatus renders the last line like statusLine does, lined up with the cards.
func (m *Model) pixelStatus() string {
	right := m.statusBadges()
	room := m.width - clabinternaltuiui.Width(right) - 1

	left := strings.Repeat(" ", pixelInset) + m.toast.view(m.th)
	if m.prompt != nil {
		left = strings.Repeat(" ", pixelInset) + m.prompt.view()
	}

	return clabinternaltuiui.PadRight(clabinternaltuiui.PadRight(left, room)+right, m.width)
}

// pixelDialog draws the surface of a dialog and takes its frame of line characters away,
// which the surface replaces.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelDialog(box string, x, y, w, h int) string {
	sc, sk := m.scene(), m.skin()
	fx, fy, fw, fh := float64(x), float64(y)-0.4, float64(w), float64(h)+0.1

	// A soft shadow, the card, and a line under the title.
	sc.Shadow(fx, fy, fw, fh, dialogRound, 22, 8, sk.Shadow)
	sc.Box(fx, fy, fw, fh, dialogRound, sk.Card, sk.CardEdge, 1)
	sc.HLine(fx, float64(y)+1, fw, sk.CardEdge)

	// Everything around the dialog is dimmed, text included.
	sc.On(clabinternaltuigfx.Over)
	sc.Punch(fx, fy, fw, fh, dialogRound)
	sc.On(clabinternaltuigfx.Under)

	lines := strings.Split(box, "\n")
	edges := strings.NewReplacer("╭", " ", "╮", " ", "╰", " ", "╯", " ", "─", " ")

	for i, line := range lines {
		switch i {
		case 0, len(lines) - 1:
			lines[i] = edges.Replace(line)
		default:
			if first := strings.Index(line, "│"); first >= 0 {
				line = line[:first] + " " + line[first+len("│"):]
			}

			if last := strings.LastIndex(line, "│"); last >= 0 {
				line = line[:last] + " " + line[last+len("│"):]
			}

			lines[i] = line
		}
	}

	return strings.Join(lines, "\n")
}

// card draws the surface the lines of a block sit on: it reaches a cell beyond them on
// each side and half a line above and below.
func (m *Model) card(sc *clabinternaltuigfx.Scene, line, lines int) {
	sk := m.skin()
	//nolint:mnd // half a line of padding
	sc.Box(
		1,
		float64(line)-0.5,
		float64(m.width)-2,
		float64(lines)+1,
		cardRound,
		sk.Card,
		sk.CardEdge,
		1,
	)
}

// pixelPane draws the card of a details pane that starts at a line of the body, and
// returns the first line of the pane: its title, and the key that hides it.
//
//nolint:mnd // sizes and positions of what is drawn, in cells and points
func (m *Model) pixelPane(title string, width, line int) string {
	if sc := m.scene(); sc != nil {
		m.card(sc, pixelBodyTop+line, paneLines)
		sc.HLine(1, float64(pixelBodyTop+line)+1, float64(m.width)-2, m.skin().CardEdge)
	}

	left := " " + m.th.Bold.Render(clabinternaltuiui.Sanitize(title))
	right := m.th.Key.Render("p") + " " + m.th.KeyDesc.Render("hide") + " "
	gap := width - clabinternaltuiui.Width(left) - clabinternaltuiui.Width(right)

	if gap < 1 {
		return clabinternaltuiui.PadRight(clabinternaltuiui.Truncate(left, width), width)
	}

	return left + strings.Repeat(" ", gap) + right
}

// indent shifts the lines of a block to the right and fills them up to a width.
func indent(block string, cols, width int) []string {
	pad := strings.Repeat(" ", cols)
	lines := strings.Split(block, "\n")

	for i := range lines {
		lines[i] = clabinternaltuiui.PadRight(pad+lines[i], width)
	}

	return lines
}

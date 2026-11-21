// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"bytes"
	"io"
	"sync"
	"time"
)

const (
	syncEnd      = "\x1b[?2026l"
	syncStart    = "\x1b[?2026h"
	altScreenOn  = "\x1b[?1049h"
	altScreenOff = "\x1b[?1049l"
	saveCursor   = "\x1b7"
	restoreCur   = "\x1b8"

	// mouseCells is what the text renderer sends to have the pointer reported in the
	// extended format, in cells. mousePixels has it reported in pixels in that format,
	// and mouseNoPixels takes that back.
	mouseCells    = "\x1b[?1006h"
	mousePixels   = "\x1b[?1016h"
	mouseNoPixels = "\x1b[?1016l"

	// lonelyDelay is how long a change of the pixel layer waits for a frame of text to
	// travel with before it is sent on its own.
	lonelyDelay = 12 * time.Millisecond
	// hurryDelay is how long it waits when the user is moving something and every
	// frame counts.
	hurryDelay = time.Millisecond
)

// File is the terminal the UI writes to.
type File interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// Output sits between the UI and the terminal. The text renderer knows nothing about
// images, so the pixel layer rides along with its frames: what changed is put in front
// of the end of the synchronized update a frame is wrapped in, and text and pixels show
// up together. It also repairs the layer when a frame scrolled or erased the screen,
// which takes the images along.
type Output struct {
	File

	mu     sync.Mutex
	screen *Screen
	// early is what is queued to go in front of the next frame of text, pending what
	// goes after it.
	early   []byte
	pending []byte
	timer   *time.Timer
	// shown is set while the terminal is on the alternate screen, the only place the
	// pixel layer is drawn.
	shown bool
	// off drops everything queued, once the UI is done with the pixel layer.
	off bool
	// mouse is set while the terminal is asked to report the pointer in pixels.
	mouse bool
}

// NewOutput wraps the terminal.
func NewOutput(f File, screen *Screen) *Output {
	return &Output{File: f, screen: screen}
}

// Queue schedules sequences of the pixel layer: the ones that take something away, which
// go in front of the next frame of text so that they cannot erase what that frame writes,
// and the ones that draw, which go after it. Without a frame they go out shortly after.
func (o *Output) Queue(before, after []byte) {
	if len(before) == 0 && len(after) == 0 {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.off {
		return
	}

	o.early = append(o.early, before...)
	o.pending = append(o.pending, after...)

	if o.timer == nil {
		o.timer = time.AfterFunc(lonelyDelay, o.flush)
	}
}

// PixelMouse asks the terminal to report the pointer in pixels rather than in cells, or
// to go back to cells. A terminal keeps one way to report the pointer, the one it was
// asked for last, and the text renderer asks for cells whenever it turns the mouse on:
// from then on the request for pixels is sent again right after each of those.
func (o *Output) PixelMouse(on bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.mouse == on || o.off {
		return
	}

	o.mouse = on

	if on {
		_, _ = o.File.Write([]byte(mousePixels))

		return
	}

	// Taking the reports in pixels back leaves a terminal with the oldest format there
	// is: it is asked for the extended one again.
	_, _ = o.File.Write([]byte(mouseNoPixels + mouseCells))
}

// Hurry has what is queued sent right away instead of waiting for a frame of text to
// travel with: for the frames of something that follows the pointer, which change the
// pixels alone.
func (o *Output) Hurry() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.timer != nil {
		o.timer.Reset(hurryDelay)
	}
}

// Stop removes the pixel layer from the terminal and drops whatever is still queued.
func (o *Output) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.off = true
	o.early, o.pending = nil, nil

	if o.shown {
		_, _ = o.File.Write(settled(guarded(o.screen.Hide())))
	}
}

// guarded wraps sequences that move the cursor or change how text is drawn, so that both
// are as they were afterwards.
func guarded(seq []byte) []byte {
	if len(seq) == 0 {
		return nil
	}

	out := make([]byte, 0, len(seq)+len(saveCursor)+len(restoreCur))
	out = append(out, saveCursor...)
	out = append(out, seq...)

	return append(out, restoreCur...)
}

// settled returns what is written to the terminal with the end of a synchronized update
// as its last bytes, adding one when it ends otherwise. It changes nothing for a terminal
// that is not in the middle of an update. It is there for the ones that hold an update
// back until the data they read ends with its end: a frame with the pixel layer is large
// and arrives in pieces, and when the end of the update comes in the same piece as what
// is written right after it, ie. the sequences that hand the terminal back on exit, such
// a terminal keeps waiting and shows nothing anymore.
func settled(p []byte) []byte {
	if bytes.HasSuffix(p, []byte(syncEnd)) {
		return p
	}

	return append(p, syncEnd...)
}

// flush sends what is queued when no frame came along to carry it.
func (o *Output) flush() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.timer = nil

	if (len(o.early) == 0 && len(o.pending) == 0) || !o.shown {
		return
	}

	var buf bytes.Buffer

	buf.WriteString(syncStart + saveCursor)
	buf.Write(o.early)
	buf.Write(o.pending)
	buf.WriteString(restoreCur + syncEnd)

	o.early, o.pending = o.early[:0], o.pending[:0]

	_, _ = o.File.Write(buf.Bytes())
}

// Write passes a frame of the text renderer on, with the pixel layer added to it.
func (o *Output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	leaves := bytes.Contains(p, []byte(altScreenOff))
	enters := bytes.Contains(p, []byte(altScreenOn))
	// The renderer just asked for the pointer in cells: see PixelMouse.
	pointer := o.mouse && !o.off && bytes.Contains(p, []byte(mouseCells))

	if o.off || (!o.shown && !enters) {
		n, err := o.File.Write(p)
		if err == nil && pointer {
			_, err = o.File.Write([]byte(mousePixels))
		}

		return n, err
	}

	var before, after []byte

	switch {
	case leaves && !enters:
		// The pixel layer does not belong on the screen the terminal goes back to.
		before = o.screen.Hide()
		o.shown = false
	case enters || disturbs(p):
		// The images are gone or out of place: put all of them back.
		before, after = o.screen.Repair()
		o.shown = true
	default:
		before, after = o.early, o.pending
		o.shown = true
	}

	var buf bytes.Buffer

	// What takes images away comes first: in a terminal that erases them with the
	// text of their cells, it would erase what the frame writes otherwise. It goes
	// inside the synchronized update of the frame when that starts with one.
	frame := p

	if bytes.HasPrefix(p, []byte(syncStart)) && len(before) > 0 {
		buf.WriteString(syncStart)

		frame = p[len(syncStart):]
	}

	buf.Write(guarded(before))

	// What draws comes last, and inside the synchronized update as well, so that the
	// terminal shows text and pixels in one go.
	switch i := bytes.LastIndex(frame, []byte(syncEnd)); {
	case len(after) == 0:
		buf.Write(frame)
	case i >= 0:
		buf.Write(frame[:i])
		buf.Write(guarded(after))
		buf.Write(frame[i:])
	default:
		buf.Write(frame)
		buf.WriteString(syncStart)
		buf.Write(guarded(after))
		buf.WriteString(syncEnd)
	}

	o.early, o.pending = o.early[:0], o.pending[:0]

	if pointer {
		buf.WriteString(mousePixels)
	}

	if _, err := o.File.Write(settled(buf.Bytes())); err != nil {
		return 0, err
	}

	return len(p), nil
}

// disturbs reports whether a frame holds a sequence that moves or erases what is on
// screen: scrolling, inserting or deleting lines, or erasing the display. Images follow
// the lines they sit on, so after such a frame they have to be placed again.
//
//nolint:mnd // the arithmetic of pixels and colors
func disturbs(p []byte) bool {
	for i := 0; i+1 < len(p); i++ {
		if p[i] != 0x1b { // ESC
			continue
		}

		switch p[i+1] {
		case 'M', 'D':
			// Reverse index and index scroll at the edge of the scroll region.
			return true
		case '[':
		default:
			continue
		}

		j := i + 2                                       // past the introducer
		for j < len(p) && (p[j] < 0x40 || p[j] > 0x7e) { // until the final byte
			j++
		}

		if j >= len(p) {
			return false
		}

		switch p[j] {
		case 'S', 'T', 'L', 'M', 'J', 'r':
			return true
		}

		i = j
	}

	return false
}

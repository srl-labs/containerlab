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

	// lonelyDelay is how long a change of the pixel layer waits for a frame of text to
	// travel with before it is sent on its own.
	lonelyDelay = 12 * time.Millisecond
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

	mu      sync.Mutex
	screen  *Screen
	pending []byte
	timer   *time.Timer
	// shown is set while the terminal is on the alternate screen, the only place the
	// pixel layer is drawn.
	shown bool
	// off drops everything queued, once the UI is done with the pixel layer.
	off bool
}

// NewOutput wraps the terminal.
func NewOutput(f File, screen *Screen) *Output {
	return &Output{File: f, screen: screen}
}

// Queue schedules sequences of the pixel layer. They go out with the next frame of text,
// or shortly after when none comes.
func (o *Output) Queue(seq []byte) {
	if len(seq) == 0 {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.off {
		return
	}

	o.pending = append(o.pending, seq...)

	if o.timer == nil {
		o.timer = time.AfterFunc(lonelyDelay, o.flush)
	}
}

// Stop removes the pixel layer from the terminal and drops whatever is still queued.
func (o *Output) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.off = true
	o.pending = nil

	if o.shown {
		_, _ = o.File.Write(settled(o.screen.Hide()))
	}
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

	if len(o.pending) == 0 || !o.shown {
		return
	}

	var buf bytes.Buffer

	buf.WriteString(syncStart + saveCursor)
	buf.Write(o.pending)
	buf.WriteString(restoreCur + syncEnd)

	o.pending = o.pending[:0]

	_, _ = o.File.Write(buf.Bytes())
}

// Write passes a frame of the text renderer on, with the pixel layer added to it.
func (o *Output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	leaves := bytes.Contains(p, []byte(altScreenOff))
	enters := bytes.Contains(p, []byte(altScreenOn))

	if o.off || (!o.shown && !enters) {
		return o.File.Write(p)
	}

	var extra []byte

	switch {
	case enters || disturbs(p):
		// The images are gone or out of place: put all of them back.
		extra = o.screen.All()
		o.pending = o.pending[:0]
	case len(o.pending) > 0:
		extra = append(extra, o.pending...)
		o.pending = o.pending[:0]
	}

	var buf bytes.Buffer

	if leaves && o.shown {
		// The pixel layer does not belong on the screen the terminal goes back to.
		buf.Write(o.screen.Hide())
	}

	switch {
	case leaves && !enters:
		o.shown = false

		buf.Write(p)
	case len(extra) == 0:
		o.shown = true

		buf.Write(p)
	default:
		o.shown = true

		// Inside the synchronized update of the frame when there is one, so that the
		// terminal shows text and pixels in one go.
		if i := bytes.LastIndex(p, []byte(syncEnd)); i >= 0 {
			buf.Write(p[:i])
			buf.WriteString(saveCursor)
			buf.Write(extra)
			buf.WriteString(restoreCur)
			buf.Write(p[i:])
		} else {
			buf.Write(p)
			buf.WriteString(syncStart + saveCursor)
			buf.Write(extra)
			buf.WriteString(restoreCur + syncEnd)
		}
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

// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package gfx

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/zlib"
)

const (
	// firstImageID is where the image numbers of the strips start. Each layer gets a
	// range of its own, one image a text row.
	firstImageID  = 31000
	layerImageIDs = 2000
	// The numbers of the images of a flat scene, which are not used again for a long
	// while.
	flatImageID  = 1_000_000
	flatImageIDs = 8_000_000
	// flatSplit is the number of rows at the top of a flat scene that are sent as an
	// image of their own, see updateFlat.
	flatSplit = 2

	// transmitKeys starts the keys of a sequence that sends an image, up to its number.
	transmitKeys = "a=T,q=2,f=32,o=z,i="

	// chunkSize is the most payload one escape sequence carries, as the protocol asks.
	chunkSize = 4096

	apcStart = "\x1b_G"
	apcEnd   = "\x1b\\"

	// blankPixel is the payload of an image of one transparent pixel.
	blankPixel = "AAAAAA=="
)

// hideSeq returns the sequence that takes a strip off the screen. It sends a blank image
// under the number of the strip without showing it, which makes the terminal drop every
// placement of the old one, also one that scrolled out of reach of a delete command, and
// then deletes that blank image.
func hideSeq(l Layer, row int) string {
	id := imageID(l, row)

	return fmt.Sprintf("%sa=t,q=2,f=32,i=%d,s=1,v=1;%s%s%sa=d,d=I,i=%d,q=2%s",
		apcStart, id, blankPixel, apcEnd, apcStart, id, apcEnd)
}

// underBackgrounds is a position in the stack of the terminal below the backgrounds of the
// cells: the protocol puts everything under half of the lowest number there.
const underBackgrounds = math.MinInt32/2 - 1

// zIndex is where the terminal stacks the images of a layer.
func (l Layer) zIndex() int {
	if l == Over {
		return 1
	}

	return underBackgrounds
}

// stripState is what the terminal shows for one text row of a layer.
type stripState struct {
	hash  uint64
	shown bool
	// cmd is the sequence that made the strip what it is, shown or hidden, kept to
	// send it again when the terminal lost or moved the images.
	cmd []byte
}

// flatRow is a text row of a flat scene as last drawn: both layers in one, as pixels
// with straight alpha.
type flatRow struct {
	hash uint64
	pix  []uint8
}

// Screen tracks what the terminal shows of the pixel layer, and turns a new scene into
// the sequences that bring the terminal up to date. It is safe for concurrent use.
type Screen struct {
	mu sync.Mutex
	// m holds the metrics of each layer as last drawn, and flat tells whether the
	// layers were merged into one image a row.
	m      [layerCount]Metrics
	flat   bool
	strips [layerCount][]stripState
	// rows holds the rows of a flat scene, and shown tells whether any of them is on
	// screen.
	rows  []flatRow
	shown bool

	// work draws the strips that are drawn one after the other, and packs the images
	// of a flat scene.
	work stripWorker
	// sent counts the images sent in the flat way, each of which gets a number of its
	// own.
	sent int
}

// stripWorker draws strips and packs them for the terminal. Each of the workers that
// draw at the same time has one.
type stripWorker struct {
	strip strip
	// top is what goes on top of the strip where both layers make one image.
	top  strip
	zbuf bytes.Buffer
	zw   *zlib.Writer
}

// stripJob is a strip that changed: what it is, and once it was drawn the sequence that
// shows it, nil for a strip nothing is visible in. A strip of a flat scene is drawn with
// both layers, into the row it is kept in.
type stripJob struct {
	l    Layer
	row  int
	draw bool
	over bool
	cmd  []byte
}

// drawStrips draws the strips that changed, as many at a time as there are processors:
// a strip depends on nothing but the scene, and while something is moved across the
// window most of them change with every frame.
func (s *Screen) drawStrips(sc *Scene, jobs []stripJob) {
	workers := min(runtime.GOMAXPROCS(0), len(jobs))
	if workers <= 1 {
		for i := range jobs {
			s.drawStrip(sc, &jobs[i], &s.work)
		}

		return
	}

	var (
		next atomic.Int64
		wg   sync.WaitGroup
	)

	for range workers {
		wg.Go(func() {
			w, _ := stripWorkers.Get().(*stripWorker)
			defer stripWorkers.Put(w)

			for {
				i := int(next.Add(1)) - 1
				if i >= len(jobs) {
					return
				}

				s.drawStrip(sc, &jobs[i], w)
			}
		})
	}

	wg.Wait()
}

var stripWorkers = sync.Pool{New: func() any { return &stripWorker{} }}

// drawStrip draws one strip and builds the sequence that shows it. It reads the screen
// without changing it.
func (s *Screen) drawStrip(sc *Scene, job *stripJob, w *stripWorker) {
	from, to := s.m[job.l].rowSpan(job.row)

	if s.flat {
		s.drawFlatRow(sc, job, w)

		return
	}

	if !job.draw || to <= from {
		return
	}

	w.strip.reset(s.m[job.l].Width, to-from, from)

	if !w.strip.render(sc.shapes[job.l]) {
		return
	}

	var cmd bytes.Buffer

	// The image is stretched over the whole row.
	m := s.m[job.l]
	fmt.Fprintf(&cmd, "\x1b[%d;1H%s%s%d,s=%d,v=%d,c=%d,r=1,z=%d,C=1", job.row+1, apcStart,
		transmitKeys, imageID(job.l, job.row), m.Width, to-from, m.Cols, job.l.zIndex())
	w.pack(&cmd, w.strip.straight())

	job.cmd = cmd.Bytes()
}

// NewScreen returns a screen that shows nothing yet.
func NewScreen() *Screen { return &Screen{} }

func imageID(l Layer, row int) int { return firstImageID + int(l)*layerImageIDs + row }

// Update returns the sequences that turn what the terminal shows into the scene: the ones
// that take away what is no longer shown, which go in front of a frame of text, and the
// ones that draw, which go after it. Both move the cursor: the caller saves and restores
// it around them.
func (s *Screen) Update(sc *Scene) (before, after []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hides, draws bytes.Buffer

	if sc.ms != s.m || sc.flat != s.flat {
		// Another window size or another way to draw: nothing of what is on screen
		// fits anymore.
		hides.Write(s.hide())

		s.m, s.flat = sc.ms, sc.flat
		for l := range s.strips {
			s.strips[l] = make([]stripState, sc.ms[l].Rows)
		}

		s.rows, s.shown = make([]flatRow, sc.ms[Under].Rows), false
	}

	if s.flat {
		s.updateFlat(&draws, sc)

		return hides.Bytes(), draws.Bytes()
	}

	var jobs []stripJob

	for l := range s.strips {
		for row := range s.strips[l] {
			st := &s.strips[l][row]
			from, to := s.m[l].rowSpan(row)

			if hash := stripHash(sc.shapes[l], from, to); hash != st.hash {
				st.hash = hash
				jobs = append(jobs, stripJob{l: Layer(l), row: row, draw: hash != 0})
			}
		}
	}

	s.drawStrips(sc, jobs)

	for i := range jobs {
		st := &s.strips[jobs[i].l][jobs[i].row]

		switch {
		case jobs[i].cmd != nil:
			st.shown, st.cmd = true, jobs[i].cmd
			draws.Write(st.cmd)
		case st.shown:
			st.shown, st.cmd = false, []byte(hideSeq(jobs[i].l, jobs[i].row))
			draws.Write(st.cmd)
		}
	}

	return hides.Bytes(), draws.Bytes()
}

// hashPrime mixes two hashes into one, as FNV does.
const hashPrime = 1099511628211

// eraseAll erases the whole screen of the terminal.
const eraseAll = "\x1b[m\x1b[2J"

// Repair returns the sequences that put the layer back after the terminal scrolled or
// erased its screen: what goes in front of the frame that did so, and what goes after it.
// Where images go with their cells, the screen is cleared first, so that the frame cannot
// move an image to a row it does not belong in.
func (s *Screen) Repair() (before, after []byte) {
	s.mu.Lock()
	flat := s.flat
	s.mu.Unlock()

	if flat {
		before = []byte(eraseAll)
	}

	return before, s.All()
}

// updateFlat brings a flat scene up to date: both layers in one image, placed on top of
// the text, for a terminal built on xterm.js.
//
// Such a terminal takes a while for every image, however small, and little more for a
// large one. And it gives an image a row more than it takes when the height of a cell
// is not a whole number of pixels, which takes the image of the row below away. So
// what changed is sent as one image, from the first row that changed down to the last
// row of the window, which has nothing below it. The rows above keep the image they
// have. An image cannot both start at the top and end at the bottom of the window, see
// flatBand: when the first rows changed they are sent on their own, and all others
// after them.
func (s *Screen) updateFlat(draws *bytes.Buffer, sc *Scene) {
	var jobs []stripJob

	for row := range s.rows {
		from, to := s.m[Under].rowSpan(row)
		under, over := stripHash(sc.shapes[Under], from, to), stripHash(sc.shapes[Over], from, to)

		hash := uint64(0)
		if under != 0 || over != 0 {
			hash = (under*hashPrime ^ over) | 1
		}

		if hash != s.rows[row].hash || s.rows[row].pix == nil {
			s.rows[row].hash = hash
			jobs = append(jobs, stripJob{row: row, draw: under != 0, over: over != 0})
		}
	}

	if len(jobs) == 0 {
		return
	}

	s.drawStrips(sc, jobs)
	s.flatFrom(draws, jobs[0].row)
}

// drawFlatRow draws a row of a flat scene into the pixels kept for it.
func (s *Screen) drawFlatRow(sc *Scene, job *stripJob, w *stripWorker) {
	from, to := s.m[Under].rowSpan(job.row)
	row := &s.rows[job.row]
	size := s.m[Under].Width * max(to-from, 0) * bytesPerPixel

	if cap(row.pix) < size {
		row.pix = make([]uint8, size)
	}

	row.pix = row.pix[:size]

	w.strip.reset(s.m[Under].Width, to-from, from)

	if job.draw {
		w.strip.render(sc.shapes[Under])
	}

	if job.over {
		w.top.reset(s.m[Under].Width, to-from, from)

		if w.top.render(sc.shapes[Over]) {
			w.strip.blend(&w.top)
		}
	}

	copy(row.pix, w.strip.straight())
}

// flatFrom writes the sequences that send the rows of a flat scene from a row on.
func (s *Screen) flatFrom(out *bytes.Buffer, row int) {
	rows := len(s.rows)
	s.shown = true

	if row >= flatSplit || rows <= flatSplit {
		s.flatBand(out, min(row, rows-1), rows)

		return
	}

	s.flatBand(out, 0, flatSplit)
	s.flatBand(out, flatSplit, rows)
}

// flatBand writes the sequence that sends some rows of a flat scene as one image, in
// the size it has: the terminal draws it with the pixels it reports, which are the ones
// the scene is drawn with.
//
// An image that ends on the last row of the window may make xterm.js move on to a row
// that is not there, which scrolls the screen, also when told to leave the cursor alone.
// It does not below the region that scrolls: the region is set to the rows above the
// image for it, of which there have to be two.
func (s *Screen) flatBand(out *bytes.Buffer, from, to int) {
	if from >= to {
		return
	}

	top, _ := s.m[Under].rowSpan(from)
	_, bottom := s.m[Under].rowSpan(to - 1)

	pix := s.work.strip.pix[:0]
	for row := from; row < to; row++ {
		pix = append(pix, s.rows[row].pix...)
	}

	s.work.strip.pix = pix

	fenced := to == len(s.rows) && from >= flatSplit
	if fenced {
		fmt.Fprintf(out, "\x1b[1;%dr", from)
	}

	s.sent++

	fmt.Fprintf(out, "\x1b[%d;1H%s%s%d,s=%d,v=%d,C=1", from+1, apcStart, transmitKeys,
		flatImageID+s.sent%flatImageIDs, s.m[Under].Width, bottom-top)
	s.work.pack(out, pix)

	if fenced {
		out.WriteString("\x1b[r")
	}
}

// stripHash identifies the look of a strip: the shapes that touch its rows, in order.
// Zero stands for an empty strip.
func stripHash(shapes []shape, from, to int) uint64 {
	h := fnv.New64a()
	touched := false

	var buf [8]byte

	for i := range shapes {
		sh := &shapes[i]
		if int(math.Ceil(float64(sh.by1))) <= from ||
			int(math.Floor(float64(sh.by0))) >= to {
			continue
		}

		touched = true

		binary.LittleEndian.PutUint64(buf[:], sh.hash)
		_, _ = h.Write(buf[:])
	}

	if !touched {
		return 0
	}

	// The pixel rows of the strip are part of how it looks: two halves of 32 bits.
	rows := uint64(from)<<32 | uint64(to) //nolint:gosec,mnd // pixel rows are not negative
	binary.LittleEndian.PutUint64(buf[:], rows)
	_, _ = h.Write(buf[:])

	return h.Sum64() | 1
}

// pack writes the pixels of an image, with straight alpha, after the keys of the sequence
// that sends it. Sending an image under a number that is in use replaces the image shown
// before.
func (w *stripWorker) pack(out *bytes.Buffer, pix []uint8) {
	w.zbuf.Reset()

	if w.zw == nil {
		w.zw, _ = zlib.NewWriterLevel(&w.zbuf, zlib.BestSpeed)
	} else {
		w.zw.Reset(&w.zbuf)
	}

	_, _ = w.zw.Write(pix)
	_ = w.zw.Close()

	payload := make([]byte, base64.StdEncoding.EncodedLen(w.zbuf.Len()))
	base64.StdEncoding.Encode(payload, w.zbuf.Bytes())

	for first := true; first || len(payload) > 0; first = false {
		chunk := payload[:min(chunkSize, len(payload))]
		payload = payload[len(chunk):]

		more := 0
		if len(payload) > 0 {
			more = 1
		}

		// An image that fits one sequence is sent without the key that tells chunks
		// apart: xterm.js shows nothing for one that says it is the last of one.
		switch {
		case first && more == 0:
			out.WriteString(";")
		case first:
			out.WriteString(",m=1;")
		default:
			fmt.Fprintf(out, "%sm=%d;", apcStart, more)
		}

		out.Write(chunk)
		out.WriteString(apcEnd)
	}
}

// All returns the sequences that put everything on screen again, for when the terminal
// dropped or moved the images.
func (s *Screen) All() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out bytes.Buffer

	for l := range s.strips {
		for row := range s.strips[l] {
			if !s.flat {
				out.Write(s.strips[l][row].cmd)
			}
		}
	}

	if s.flat && s.shown {
		s.flatFrom(&out, 0)
	}

	return out.Bytes()
}

// Hide returns the sequences that take the pixel layer off the screen. What was shown is
// remembered: All brings it back.
func (s *Screen) Hide() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hide()
}

func (s *Screen) hide() []byte {
	var out bytes.Buffer

	// A terminal that shows a placeholder for an image it is told to delete is
	// cleared as a whole: its images go with the cells they sit in, and none of them
	// holds text, which is all written by the layer there.
	if s.flat {
		return []byte(eraseAll)
	}

	for l := range s.strips {
		for row := range s.strips[l] {
			if s.strips[l][row].shown {
				out.WriteString(hideSeq(Layer(l), row))
			}
		}
	}

	return out.Bytes()
}

// Forget makes the screen assume the terminal shows nothing, so that the next update
// sends every strip.
func (s *Screen) Forget() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.m, s.flat = [layerCount]Metrics{}, false
	s.strips = [layerCount][]stripState{}
	s.rows, s.shown = nil, false
}

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
	"strconv"
	"sync"

	"github.com/klauspost/compress/zlib"
)

const (
	// firstImageID is where the image numbers of the strips start. Each layer gets a
	// range of its own, one image a text row.
	firstImageID  = 31000
	layerImageIDs = 2000
	// The numbers of the images of a flat scene, which are not used again for a long
	// while. All of them have as many digits, so that one can take the place of
	// another in a sequence that is sent again.
	flatImageID  = 1_000_000
	flatImageIDs = 8_000_000

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
	// send it again when the terminal lost or moved the images, and idAt where the
	// number of the image stands in it, for a flat scene.
	cmd  []byte
	idAt int
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

	strip strip
	top   strip
	// sent counts the images sent in the flat way, each of which gets a number of its
	// own.
	sent int
	zbuf bytes.Buffer
	zw   *zlib.Writer
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
	}

	if s.flat {
		// xterm.js gives an image a row more than it takes, and so wipes the row
		// below it: a row under one that was sent is sent again, down to the first
		// row that shows nothing.
		wiped := false
		for row := range s.strips[Under] {
			wiped = s.updateFlat(&hides, &draws, sc, row, wiped)
		}

		return hides.Bytes(), draws.Bytes()
	}

	for l := range s.strips {
		for row := range s.strips[l] {
			s.updateStrip(&draws, sc, Layer(l), row)
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

// eraseRow returns the sequence that erases a row of the terminal, which is how an image
// is taken off a terminal that shows a placeholder for an image it was told to delete.
func eraseRow(row int) string { return fmt.Sprintf("\x1b[m\x1b[%d;1H\x1b[2K", row+1) }

// updateFlat brings one row of a flat scene up to date: both layers in one image, placed
// on top of the text. wiped tells that the row above was sent, which took the image of
// this one away; it reports whether this row was sent.
func (s *Screen) updateFlat(hides, draws *bytes.Buffer, sc *Scene, row int, wiped bool) bool {
	st := &s.strips[Under][row]
	from, to := s.m[Under].rowSpan(row)

	under, over := stripHash(sc.shapes[Under], from, to), stripHash(sc.shapes[Over], from, to)

	hash := uint64(0)
	if under != 0 || over != 0 {
		hash = (under*hashPrime ^ over) | 1
	}

	if hash == st.hash {
		if wiped && st.shown {
			draws.Write(s.again(st))
		}

		return wiped && st.shown
	}

	st.hash = hash
	painted := false

	if hash != 0 && to > from {
		s.strip.reset(s.m[Under].Width, to-from, from)
		painted = s.strip.render(sc.shapes[Under])

		if over != 0 {
			s.top.reset(s.m[Under].Width, to-from, from)

			if s.top.render(sc.shapes[Over]) {
				s.strip.blend(&s.top)

				painted = true
			}
		}
	}

	if !painted {
		if st.shown {
			hides.WriteString(eraseRow(row))
		}

		st.shown, st.cmd = false, nil

		return false
	}

	var cmd bytes.Buffer

	// On its last row xterm.js scrolls the screen up a line when it is given an
	// image, also when told to leave the cursor alone. A row below the region that
	// scrolls does not: the region is taken off the last row for that image.
	last := row == len(s.strips[Under])-1 && row > 0
	if last {
		fmt.Fprintf(&cmd, "\x1b[1;%dr", row)
	}

	fmt.Fprintf(&cmd, "\x1b[%d;1H", row+1)
	st.idAt = cmd.Len() + len(apcStart) + len(transmitKeys)
	s.transmit(&cmd, flatImageID, Under, to-from, 0)

	if last {
		cmd.WriteString("\x1b[r")
	}

	st.shown, st.cmd = true, cmd.Bytes()
	draws.Write(s.again(st))

	return true
}

// again returns the sequence that sends the image of a flat row, under a number of its
// own each time. Sending an image under a number that is in use while the terminal is
// still busy with the one before leaves a placeholder in xterm.js; an image that is new
// just takes the cells over.
func (s *Screen) again(st *stripState) []byte {
	s.sent++

	id := strconv.Itoa(flatImageID + s.sent%flatImageIDs)
	copy(st.cmd[st.idAt:st.idAt+len(id)], id)

	return st.cmd
}

func (s *Screen) updateStrip(out *bytes.Buffer, sc *Scene, l Layer, row int) {
	st := &s.strips[l][row]
	from, to := s.m[l].rowSpan(row)
	hash := stripHash(sc.shapes[l], from, to)

	if hash == st.hash {
		return
	}

	st.hash = hash
	painted := false

	if hash != 0 && to > from {
		s.strip.reset(s.m[l].Width, to-from, from)
		painted = s.strip.render(sc.shapes[l])
	}

	if !painted {
		if st.shown {
			st.shown, st.cmd = false, []byte(hideSeq(l, row))
			out.Write(st.cmd)
		}

		return
	}

	var cmd bytes.Buffer

	fmt.Fprintf(&cmd, "\x1b[%d;1H", row+1)
	s.transmit(&cmd, imageID(l, row), l, to-from, l.zIndex())

	st.shown, st.cmd = true, cmd.Bytes()
	out.Write(st.cmd)
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

// transmit writes the sequences that send the rendered strip and show it at the cursor,
// stretched over the whole row. Sending an image under a number that is in use replaces
// the image shown before.
func (s *Screen) transmit(out *bytes.Buffer, id int, l Layer, height, z int) {
	s.zbuf.Reset()

	if s.zw == nil {
		s.zw, _ = zlib.NewWriterLevel(&s.zbuf, zlib.BestSpeed)
	} else {
		s.zw.Reset(&s.zbuf)
	}

	_, _ = s.zw.Write(s.strip.straight())
	_ = s.zw.Close()

	payload := make([]byte, base64.StdEncoding.EncodedLen(s.zbuf.Len()))
	base64.StdEncoding.Encode(payload, s.zbuf.Bytes())

	fmt.Fprintf(out, "%s%s%d,s=%d,v=%d,c=%d,r=1,z=%d,C=1",
		apcStart, transmitKeys, id, s.m[l].Width, height, s.m[l].Cols, z)

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
			st := &s.strips[l][row]

			switch {
			case s.flat && st.shown:
				out.Write(s.again(st))
			case !s.flat:
				out.Write(st.cmd)
			}
		}
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
}

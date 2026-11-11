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
	"sync"

	"github.com/klauspost/compress/zlib"
)

const (
	// firstImageID is where the image numbers of the strips start. Each layer gets a
	// range of its own, one image a text row.
	firstImageID  = 31000
	layerImageIDs = 2000

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

// Screen tracks what the terminal shows of the pixel layer, and turns a new scene into
// the sequences that bring the terminal up to date. It is safe for concurrent use.
type Screen struct {
	mu sync.Mutex
	// m holds the metrics of each layer as last drawn.
	m      [layerCount]Metrics
	strips [layerCount][]stripState

	strip strip
	zbuf  bytes.Buffer
	zw    *zlib.Writer
}

// NewScreen returns a screen that shows nothing yet.
func NewScreen() *Screen { return &Screen{} }

func imageID(l Layer, row int) int { return firstImageID + int(l)*layerImageIDs + row }

// Update returns the sequences that turn what the terminal shows into the scene. They
// move the cursor: the caller saves and restores it around them.
func (s *Screen) Update(sc *Scene) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out bytes.Buffer

	if sc.ms != s.m {
		// Another window size: nothing of what is on screen fits anymore.
		out.Write(s.hide())

		s.m = sc.ms
		for l := range s.strips {
			s.strips[l] = make([]stripState, sc.ms[l].Rows)
		}
	}

	for l := range s.strips {
		for row := range s.strips[l] {
			s.updateStrip(&out, sc, Layer(l), row)
		}
	}

	return out.Bytes()
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
	s.transmit(&cmd, l, row, to-from)

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
func (s *Screen) transmit(out *bytes.Buffer, l Layer, row, height int) {
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

	fmt.Fprintf(out, "%sa=T,q=2,f=32,o=z,i=%d,s=%d,v=%d,c=%d,r=1,z=%d,C=1",
		apcStart, imageID(l, row), s.m[l].Width, height, s.m[l].Cols, l.zIndex())

	for first := true; first || len(payload) > 0; first = false {
		chunk := payload[:min(chunkSize, len(payload))]
		payload = payload[len(chunk):]

		more := 0
		if len(payload) > 0 {
			more = 1
		}

		if first {
			fmt.Fprintf(out, ",m=%d;", more)
		} else {
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
			out.Write(s.strips[l][row].cmd)
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

	s.m = [layerCount]Metrics{}
	s.strips = [layerCount][]stripState{}
}

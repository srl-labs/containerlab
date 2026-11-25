// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// The parts of a drawing file that hold the positions of the nodes.
	nodeAnnotationsKey = "nodeAnnotations"
	annotationIDKey    = "id"
	positionKey        = "position"

	// drawingIndent is how the containerlab application indents a drawing file.
	drawingIndent = "  "
	// drawingMode is the mode of a drawing file that is created.
	drawingMode = 0o644
	// positionDigits is the number of decimals a position is saved with.
	positionDigits = 2
)

// PlaceNodes implements clabinternaltuibackend.Backend. The positions go into the file the
// containerlab application keeps its drawing of the topology in, next to the topology
// file, so that both show the same picture.
func (b *Backend) PlaceNodes(
	lab string,
	positions map[string]clabinternaltuibackend.Position,
) error {
	if b.opts.ReadOnly {
		return fmt.Errorf("%w: %s", clabinternaltuibackend.ErrUnavailable, reasonReadOnly)
	}

	if len(positions) == 0 {
		return nil
	}

	topoFile, err := b.drawnTopology(lab)
	if err != nil {
		return err
	}

	b.drawMu.Lock()
	defer b.drawMu.Unlock()

	if err := placeNodes(topoFile+annotationsSuffix, topoFile, positions); err != nil {
		return err
	}

	// The topology is read again, and with it the positions.
	b.mark(clabinternaltuibackend.TopicLinks)

	return nil
}

// drawnTopology returns the topology file of a lab, which its drawing is kept next to.
func (b *Backend) drawnTopology(lab string) (string, error) {
	if rec := b.inv.record(lab); rec != nil {
		if rec.lab.TopoFile == "" || rec.lab.TopoMissing {
			return "", fmt.Errorf("%w: the topology file of %s is not on this host",
				clabinternaltuibackend.ErrUnavailable, lab)
		}

		return rec.lab.TopoFile, nil
	}

	if file, ok := b.files.byName(lab); ok {
		return file.path, nil
	}

	return "", fmt.Errorf("unknown lab %s", lab)
}

// placeNodes writes positions into a drawing file. Everything else in the file is kept
// as it is, in the order it is in, so that the file changes no more than it does when the
// containerlab application moves a node.
func placeNodes(
	path, topoFile string,
	positions map[string]clabinternaltuibackend.Position,
) error {
	var doc jsonObject

	trailer := ""

	data, err := readLimited(path)

	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	default:
		if doc, err = parseJSONObject(data); err != nil {
			return fmt.Errorf("%s is left as it is, it does not hold a drawing: %w", path, err)
		}

		if bytes.HasSuffix(data, []byte("\n")) {
			trailer = "\n"
		}
	}

	nodes, err := placedAnnotations(doc.get(nodeAnnotationsKey), positions)
	if err != nil {
		return fmt.Errorf("%s is left as it is: %w", path, err)
	}

	doc.set(nodeAnnotationsKey, nodes)

	var out bytes.Buffer
	if err := json.Indent(&out, doc.encode(), "", drawingIndent); err != nil {
		return err
	}

	out.WriteString(trailer)

	if bytes.Equal(out.Bytes(), data) {
		return nil
	}

	return replaceFile(path, topoFile, out.Bytes())
}

// placedAnnotations returns the list of node annotations with the positions set: in the
// annotation of a node that has one, and in a new one at the end otherwise.
func placedAnnotations(
	list json.RawMessage,
	positions map[string]clabinternaltuibackend.Position,
) (json.RawMessage, error) {
	var items []json.RawMessage

	if len(list) > 0 && !bytes.Equal(bytes.TrimSpace(list), []byte("null")) {
		if err := json.Unmarshal(list, &items); err != nil {
			return nil, fmt.Errorf("its %s are not a list", nodeAnnotationsKey)
		}
	}

	placed := map[string]bool{}

	for i, item := range items {
		node, err := parseJSONObject(item)
		if err != nil {
			// Not an annotation this code knows how to read: it stays.
			continue
		}

		var id string
		if json.Unmarshal(node.get(annotationIDKey), &id) != nil {
			continue
		}

		pos, ok := positions[id]
		if !ok || placed[id] {
			continue
		}

		placed[id] = true

		node.set(positionKey, positionJSON(pos))
		items[i] = node.encode()
	}

	for _, id := range slices.Sorted(maps.Keys(positions)) {
		if placed[id] {
			continue
		}

		name, err := json.Marshal(id)
		if err != nil {
			return nil, err
		}

		var node jsonObject

		node.set(annotationIDKey, name)
		node.set(positionKey, positionJSON(positions[id]))
		items = append(items, node.encode())
	}

	var out bytes.Buffer

	out.WriteByte('[')

	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}

		out.Write(item)
	}

	out.WriteByte(']')

	return out.Bytes(), nil
}

// positionJSON writes a position the way the containerlab application does: whole
// numbers without decimals.
func positionJSON(pos clabinternaltuibackend.Position) json.RawMessage {
	number := func(v float64) string {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}

		scale := math.Pow10(positionDigits)

		return strconv.FormatFloat(math.Round(v*scale)/scale, 'f', -1, 64)
	}

	return json.RawMessage(`{"x":` + number(pos.X) + `,"y":` + number(pos.Y) + `}`)
}

// jsonMember is a member of a JSON object, its value as it was written.
type jsonMember struct {
	key   string
	value json.RawMessage
}

// jsonObject is a JSON object that keeps the order of its members and what it does not
// know about them, so that it can be written back with a single member changed.
type jsonObject []jsonMember

// parseJSONObject reads a JSON object. Anything else is an error.
func parseJSONObject(data []byte) (jsonObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	if tok, err := dec.Token(); err != nil {
		return nil, err
	} else if tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}

	var obj jsonObject

	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		key, ok := tok.(string)
		if !ok {
			return nil, errors.New("not a JSON object")
		}

		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}

		obj = append(obj, jsonMember{key: key, value: value})
	}

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	// Nothing but blanks may follow the object.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one JSON value")
	}

	return obj, nil
}

// get returns the value of a member, nil when the object has none of that name.
func (o *jsonObject) get(key string) json.RawMessage {
	for i := range *o {
		if (*o)[i].key == key {
			return (*o)[i].value
		}
	}

	return nil
}

// set changes the value of a member, or adds the member at the end.
func (o *jsonObject) set(key string, value json.RawMessage) {
	for i := range *o {
		if (*o)[i].key == key {
			(*o)[i].value = value

			return
		}
	}

	*o = append(*o, jsonMember{key: key, value: value})
}

// encode writes the object on one line.
func (o *jsonObject) encode() json.RawMessage {
	var out bytes.Buffer

	out.WriteByte('{')

	for i, member := range *o {
		if i > 0 {
			out.WriteByte(',')
		}

		// A string always encodes: this is for the linter.
		key, err := json.Marshal(member.key)
		if err != nil {
			continue
		}

		out.Write(key)
		out.WriteByte(':')
		out.Write(member.value)
	}

	out.WriteByte('}')

	return out.Bytes()
}

// replaceFile puts new content in a file in one step, so that a reader never finds half
// of it. The file keeps its mode, and its owner when this process may set it: a drawing
// written with root privileges still belongs to the user whose topology it draws.
func replaceFile(path, like string, content []byte) error {
	mode := os.FileMode(drawingMode)
	owner := like

	if info, err := os.Stat(path); err == nil {
		mode, owner = info.Mode().Perm(), path
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		// The directory may not take new files while the file itself can be written.
		return os.WriteFile(path, content, mode)
	}

	name := tmp.Name()
	keep := false

	defer func() {
		if !keep {
			_ = os.Remove(name)
		}
	}()

	_, err = tmp.Write(content)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return err
	}

	if err := os.Chmod(name, mode); err != nil {
		return err
	}

	if info, err := os.Stat(owner); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			// Only root can give a file away: for everyone else it is theirs anyway.
			_ = os.Chown(name, int(stat.Uid), int(stat.Gid))
		}
	}

	if err := os.Rename(name, path); err != nil {
		return err
	}

	keep = true

	return nil
}

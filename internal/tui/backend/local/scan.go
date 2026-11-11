// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"cmp"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	"gopkg.in/yaml.v2"
)

const (
	// The scan for topology files gives up beyond these numbers, so that a start in a huge
	// directory tree stays cheap.
	scanMaxDirs  = 5000
	scanMaxFiles = 500
	// scanShallowDepth is the depth used below the home and the root directory, which are
	// too large to search deeper.
	scanShallowDepth = 1

	// stateFile marks a lab directory, as does the topology export.
	stateFile = ".state.clab.yaml"
	// clabHomeDir is where containerlab keeps the labs it fetched, below the home directory.
	clabHomeDir = ".clab"
)

var (
	topologySuffixes = []string{".clab.yml", ".clab.yaml"}
	// scanSkippedDirs are directories that never hold topologies worth listing.
	scanSkippedDirs = []string{"node_modules", "__pycache__", "vendor"}
)

// topoFile is a topology file found on disk.
type topoFile struct {
	// path is the absolute path of the file, real is the path with symbolic links resolved.
	path, real string
	// name is the name of the lab the file defines.
	name string
	// nodes is the number of nodes the file defines, zero when unknown.
	nodes int
}

// belongsTo reports whether the file is the topology file of a deployed lab. The name of
// the lab only decides for a lab whose own file is gone, ie. was moved: two files may well
// define labs of the same name.
func (f *topoFile) belongsTo(rec *labRecord) bool {
	lab := &rec.lab

	if lab.TopoMissing {
		return f.name == lab.Name
	}

	return f.path == filepath.Clean(lab.TopoFile) || (f.real != "" && f.real == rec.topoReal)
}

// scanRoot is a directory to search for topology files down to a depth.
type scanRoot struct {
	dir   string
	depth int
}

// scanRoots assembles the directories to search from the given candidates. Directories
// that do not exist are dropped, as are duplicates, also when reached through symbolic
// links. The home and the root directory are only searched one level deep.
func scanRoots(dirs []string, home string, depth int) []scanRoot {
	var roots []scanRoot

	seen := map[string]bool{}
	homeReal := realPath(home)

	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if dir == "" || err != nil {
			continue
		}

		resolved := realPath(abs)
		if resolved == "" || seen[resolved] {
			continue
		}

		seen[resolved] = true
		root := scanRoot{dir: abs, depth: depth}

		if resolved == homeReal || resolved == string(filepath.Separator) {
			root.depth = min(depth, scanShallowDepth)
		}

		roots = append(roots, root)
	}

	return roots
}

// scanLimits bounds a scan.
type scanLimits struct {
	dirs, files int
}

// findTopologyFiles walks the roots and returns the topology files below them. It reports
// whether it stopped at a limit. Hidden directories, lab directories and directories of
// third party code are not entered, and symbolic links to directories are not followed.
func findTopologyFiles(
	ctx context.Context,
	roots []scanRoot,
	limits scanLimits,
) (files []string, truncated bool) {
	queue := slices.Clone(roots)
	visited := 0

	for len(queue) > 0 && ctx.Err() == nil {
		current := queue[0]
		queue = queue[1:]

		if visited >= limits.dirs {
			return files, true
		}

		visited++

		entries, err := os.ReadDir(current.dir)
		if err != nil || isLabDir(entries) {
			continue
		}

		for _, entry := range entries {
			path := filepath.Join(current.dir, entry.Name())

			switch {
			case entry.IsDir():
				if current.depth > 0 && !isSkippedDir(entry.Name()) {
					queue = append(queue, scanRoot{dir: path, depth: current.depth - 1})
				}
			case isTopologyFile(entry, path):
				if len(files) >= limits.files {
					return files, true
				}

				files = append(files, path)
			}
		}
	}

	return files, false
}

// isLabDir reports whether the entries are those of the directory of a deployed lab. What
// is in there are copies and generated files.
func isLabDir(entries []fs.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() == topologyDataFile || entry.Name() == stateFile {
			return true
		}
	}

	return false
}

func isSkippedDir(name string) bool {
	return strings.HasPrefix(name, ".") || slices.Contains(scanSkippedDirs, name)
}

// isTopologyFile reports whether a directory entry is a topology file, going by its name.
// A symbolic link counts when it leads to a regular file.
func isTopologyFile(entry fs.DirEntry, path string) bool {
	if !hasTopologySuffix(entry.Name()) {
		return false
	}

	if entry.Type().IsRegular() {
		return true
	}

	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}

	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

func hasTopologySuffix(name string) bool {
	for _, suffix := range topologySuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
}

// topologyHeader is the little of a topology file the list of labs needs.
type topologyHeader struct {
	Name     string `yaml:"name"`
	Topology struct {
		Nodes map[string]any `yaml:"nodes"`
	} `yaml:"topology"`
}

// readTopoFile reads the lab name and the number of nodes of a topology file. The file is
// only decoded as YAML, not parsed as a topology, which would be far too slow for a scan. A
// file that cannot be decoded, ie. one that is a template, is listed under its file name.
func readTopoFile(path string) topoFile {
	file := topoFile{path: path, real: realPath(path), name: labNameFromFile(path)}

	data, err := readLimited(path)
	if err != nil {
		return file
	}

	var header topologyHeader
	if err := yaml.Unmarshal(data, &header); err != nil {
		return file
	}

	if name := clabinternaltuitermtext.Sanitize(strings.TrimSpace(header.Name)); name != "" {
		file.name = name
	}

	file.nodes = len(header.Topology.Nodes)

	return file
}

// labNameFromFile derives a lab name from the name of a topology file.
func labNameFromFile(path string) string {
	name := filepath.Base(path)

	for _, suffix := range topologySuffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	return name
}

// topoFiles holds the topology files found by the last scan.
type topoFiles struct {
	mu    sync.RWMutex
	files []topoFile
	// wanted is signaled to ask for a new scan.
	wanted chan struct{}
}

// request asks for a new scan. It never blocks.
func (t *topoFiles) request() {
	select {
	case t.wanted <- struct{}{}:
	default:
	}
}

// list returns the files of the last scan. The slice is shared and must not be changed.
func (t *topoFiles) list() []topoFile {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.files
}

// byName finds the file of a lab by the lab's name.
func (t *topoFiles) byName(lab string) (topoFile, bool) {
	for _, file := range t.list() {
		if file.name == lab {
			return file, true
		}
	}

	return topoFile{}, false
}

// replace stores the result of a scan and reports whether it differs from the last one.
func (t *topoFiles) replace(files []topoFile) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	changed := !slices.Equal(t.files, files)
	t.files = files

	return changed
}

// scanLoop looks for topology files whenever a scan is requested, until ctx ends. A scan
// runs on its own, so that a slow file system holds nothing else up.
func (b *Backend) scanLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.files.wanted:
		}

		files := b.scanTopologies(ctx)
		if ctx.Err() != nil {
			return
		}

		if b.files.replace(files) {
			b.mark(clabinternaltuibackend.TopicLabs)
		}
	}
}

// scanTopologies searches the places topology files are expected in: the working
// directory, the directories of the deployed labs, the configured directories and the
// directory containerlab keeps fetched labs in.
func (b *Backend) scanTopologies(ctx context.Context) []topoFile {
	records := b.inv.records()
	home, _ := os.UserHomeDir()

	dirs := []string{"."}

	for _, rec := range records {
		if rec.lab.TopoFile != "" {
			dirs = append(dirs, filepath.Dir(rec.lab.TopoFile))
		}
	}

	dirs = append(dirs, b.opts.LabsDirs...)

	if home != "" {
		dirs = append(dirs, filepath.Join(home, clabHomeDir))
	}

	paths, truncated := findTopologyFiles(ctx, scanRoots(dirs, home, b.opts.ScanDepth),
		scanLimits{dirs: scanMaxDirs, files: scanMaxFiles})
	if truncated && !b.scanTruncated {
		b.logf(levelWarn, "the search for topology files stopped at its limit of %d "+
			"directories or %d files, some files may not be listed", scanMaxDirs, scanMaxFiles)
	}

	b.scanTruncated = truncated

	// The files of the deployed labs are known without searching for them.
	for _, rec := range records {
		if !rec.lab.TopoMissing {
			paths = append(paths, rec.lab.TopoFile)
		}
	}

	return readTopoFiles(ctx, paths)
}

// readTopoFiles reads the given topology files, each once however many paths lead to it,
// and returns them ordered by lab name.
func readTopoFiles(ctx context.Context, paths []string) []topoFile {
	files := make([]topoFile, 0, len(paths))
	seen := map[string]bool{}

	for _, path := range paths {
		if ctx.Err() != nil {
			break
		}

		file := readTopoFile(path)

		id := cmp.Or(file.real, file.path)
		if seen[id] {
			continue
		}

		seen[id] = true

		files = append(files, file)
	}

	slices.SortFunc(files, func(a, b topoFile) int {
		return cmp.Or(
			clabinternaltuitermtext.NaturalCompare(a.name, b.name),
			strings.Compare(a.path, b.path),
		)
	})

	return files
}

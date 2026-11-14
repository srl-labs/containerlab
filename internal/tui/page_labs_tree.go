// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package tui

import (
	"path/filepath"
	"slices"
	"strings"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	clabinternaltuiui "github.com/srl-labs/containerlab/internal/tui/ui"
)

// labFolder is a directory holding topology files that are not deployed, shown in the
// list of labs as a folder that opens and closes.
type labFolder struct {
	// path is the directory. name is what the list calls it: the path below the folder
	// it is in, or the whole path for a folder at the top.
	path, name string
	// labs are the labs whose files are in the directory, as indices into the labs of
	// the model, and folders the directories below it.
	labs    []int
	folders []*labFolder
	// count is the number of labs in the folder and the folders below it.
	count int
	// order sorts the folders at the top: the directory the UI was started in is the
	// top itself, the folders below it come first, then the other directories searched
	// in the order they were given, then whatever lies elsewhere.
	order int
}

// folderKey returns the key of the row of a folder. It cannot be the key of a lab.
func folderKey(path string) string {
	return "\x00dir\x00" + path
}

// labTree arranges the labs that are not deployed in the directories their files are in.
// The returned folder is the directory the UI was started in: the labs and folders in it
// are shown at the top. A directory that was searched besides it is a folder at the top,
// and so is the directory of a file found elsewhere. A folder that holds just another
// folder is shown as one, with both names.
func labTree(labs []clabinternaltuibackend.Lab, dirs []string, home string) *labFolder {
	top := &labFolder{}
	if len(dirs) > 0 {
		top.path = dirs[0]
	}

	byPath := map[string]*labFolder{}

	// folder returns the folder of a directory below a parent, creating it.
	folder := func(parent *labFolder, path, name string, order int) *labFolder {
		if f, ok := byPath[path]; ok {
			return f
		}

		f := &labFolder{path: path, name: name, order: order}
		byPath[path] = f
		parent.folders = append(parent.folders, f)

		return f
	}

	for i := range labs {
		lab := &labs[i]
		if lab.Deployed {
			continue
		}

		if lab.TopoFile == "" {
			top.labs = append(top.labs, i)

			continue
		}

		dir := filepath.Dir(lab.TopoFile)
		root, order := searchedDir(dir, dirs)

		f := top
		if root != top.path || top.path == "" {
			f = folder(top, root, clabinternaltuiui.HomePath(root, home), order)
		}

		if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
			for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
				f = folder(f, filepath.Join(f.path, part), part, 0)
			}
		}

		f.labs = append(f.labs, i)
	}

	for _, f := range top.folders {
		f.compact()
	}

	top.tally()
	top.sort(labs)

	return top
}

// searchedDir returns the deepest of the searched directories a directory is in, and its
// place among them counted from one. A directory in none of them is its own, placed after
// them.
func searchedDir(dir string, dirs []string) (string, int) {
	best, order := "", 0
	sep := string(filepath.Separator)

	for k, d := range dirs {
		in := dir == d || strings.HasPrefix(dir, strings.TrimSuffix(d, sep)+sep)
		if in && len(d) > len(best) {
			best, order = d, k+1
		}
	}

	if best == "" {
		return dir, len(dirs) + 1
	}

	return best, order
}

// compact merges a folder that holds nothing but one folder with that folder, all the
// way down, and then does the same below.
func (f *labFolder) compact() {
	for len(f.labs) == 0 && len(f.folders) == 1 {
		only := f.folders[0]
		f.name = filepath.Join(f.name, only.name)
		f.path = only.path
		f.labs = only.labs
		f.folders = only.folders
	}

	for _, sub := range f.folders {
		sub.compact()
	}
}

// tally counts the labs in the folder and the folders below it.
func (f *labFolder) tally() int {
	f.count = len(f.labs)

	for _, sub := range f.folders {
		f.count += sub.tally()
	}

	return f.count
}

// sort orders the labs of the folder by name and its folders by their order, then name.
func (f *labFolder) sort(labs []clabinternaltuibackend.Lab) {
	slices.SortStableFunc(f.labs, func(a, b int) int {
		return clabinternaltuitermtext.NaturalCompare(labs[a].Name, labs[b].Name)
	})

	slices.SortStableFunc(f.folders, func(a, b *labFolder) int {
		if a.order != b.order {
			return a.order - b.order
		}

		return clabinternaltuitermtext.NaturalCompare(a.name, b.name)
	})

	for _, sub := range f.folders {
		sub.sort(labs)
	}
}

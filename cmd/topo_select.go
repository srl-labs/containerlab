package cmd

import (
	"context"
	"path/filepath"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabutilsselector "github.com/srl-labs/containerlab/utils/selector"
)

// selectTopoFile shows an interactive picker for choosing among multiple
// discovered topology files, annotating the ones whose absolute path matches a
// currently deployed lab. It returns the selected path.
func selectTopoFile(files []string, running map[string]bool) (string, error) {
	items := make([]clabutilsselector.Item, len(files))

	for i, f := range files {
		note := ""
		if abs, err := filepath.Abs(f); err == nil && running[abs] {
			note = "(running)"
		}

		items[i] = clabutilsselector.Item{Label: f, Note: note}
	}

	idx, err := clabutilsselector.FromList("Multiple topology files found, select one:", items)
	if err != nil {
		return "", err
	}

	return files[idx], nil
}

// runningTopoFiles returns the set of absolute topology-file paths that currently
// have deployed containers (keyed by the clab-topo-file label). It is best-effort:
// on any error (e.g. the runtime is unavailable) it returns an empty set so the
// picker still works, just without "(running)" annotations.
func runningTopoFiles(ctx context.Context, o *Options) map[string]bool {
	running := map[string]bool{}

	clab, err := clabcore.NewContainerLab(o.Global.toClabOptions()...)
	if err != nil {
		return running
	}

	containers, err := clab.ListContainers(ctx, clabcore.WithListclabLabelExists())
	if err != nil {
		return running
	}

	for i := range containers {
		if topo := containers[i].Labels[clabconstants.TopoFile]; topo != "" {
			running[topo] = true
		}
	}

	return running
}

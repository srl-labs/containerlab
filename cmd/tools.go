// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	clablabels "github.com/srl-labs/containerlab/labels"
)

func toolsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "tools",
		Short: "various tools your lab might need",
		Long:  "tools command groups various tools you might need for your lab\nreference: https://containerlab.dev/cmd/tools/",
	}

	c.AddCommand(
		disableTxOffloadCmd(),
		gottyCmd(),
		sshxCmd(),
		apiServerCmd(),
		certCmd(),
		netemCmd(),
		vethCmd(),
		vxlanCmd(),
	)

	return c
}

// createLabelsMap creates container labels map for additional containers launched after the lab
// is up. Such as sshx, gotty, etc.
func createLabelsMap(topo, labName, containerName, owner, toolType string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		clablabels.Containerlab: labName,
		clablabels.NodeName:     shortName,
		clablabels.LongName:     containerName,
		clablabels.NodeKind:     "linux",
		clablabels.NodeGroup:    "",
		clablabels.NodeType:     "tool",
		clablabels.ToolType:     toolType,
	}

	// Add topology file path
	if topo != "" {
		absPath, err := filepath.Abs(topo)
		if err == nil {
			labels[clablabels.TopoFile] = absPath
		} else {
			labels[clablabels.TopoFile] = topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(topo)
		labels[clablabels.NodeLabDir] = filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[clablabels.Owner] = owner
	}

	return labels
}

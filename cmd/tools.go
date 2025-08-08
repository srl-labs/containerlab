// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	containerlablabels "github.com/srl-labs/containerlab/labels"
)

// toolsCmd represents the tools command.
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "various tools your lab might need",
	Long:  "tools command groups various tools you might need for your lab\nreference: https://containerlab.dev/cmd/tools/",
}

func init() {
	RootCmd.AddCommand(toolsCmd)
}

// createLabelsMap creates container labels map for additional containers launched after the lab
// is up. Such as sshx, gotty, etc.
func createLabelsMap(topo, labName, containerName, owner, toolType string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		containerlablabels.Containerlab: labName,
		containerlablabels.NodeName:     shortName,
		containerlablabels.LongName:     containerName,
		containerlablabels.NodeKind:     "linux",
		containerlablabels.NodeGroup:    "",
		containerlablabels.NodeType:     "tool",
		containerlablabels.ToolType:     toolType,
	}

	// Add topology file path
	if topo != "" {
		absPath, err := filepath.Abs(topo)
		if err == nil {
			labels[containerlablabels.TopoFile] = absPath
		} else {
			labels[containerlablabels.TopoFile] = topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(topo)
		labels[containerlablabels.NodeLabDir] =
			filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[containerlablabels.Owner] = owner
	}

	return labels
}

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

func toolsSubcommandRegisterFuncs() []func(*Options) (*cobra.Command, error) {
	return []func(*Options) (*cobra.Command, error){
		apiServerCmd,
		certCmd,
		disableTxOffloadCmd,
		gottyCmd,
		netemCmd,
		sshxCmd,
		vethCmd,
		vxlanCmd,
	}
}

func toolsCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "tools",
		Short: "various tools your lab might need",
		Long: "tools command groups various tools you might need for your lab\n" +
			"reference: https://containerlab.dev/cmd/tools/",
	}

	for _, f := range toolsSubcommandRegisterFuncs() {
		cmd, err := f(o)
		if err != nil {
			return nil, err
		}

		c.AddCommand(cmd)
	}

	return c, nil
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

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
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
		codeServerCmd,
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
		clabconstants.Containerlab: labName,
		clabconstants.NodeName:     shortName,
		clabconstants.LongName:     containerName,
		clabconstants.NodeKind:     "linux",
		clabconstants.NodeGroup:    "",
		clabconstants.NodeType:     "tool",
		clabconstants.ToolType:     toolType,
	}

	// Add topology file path
	if topo != "" {
		absPath, err := filepath.Abs(topo)
		if err == nil {
			labels[clabconstants.TopoFile] = absPath
		} else {
			labels[clabconstants.TopoFile] = topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(topo)
		labels[clabconstants.NodeLabDir] = filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[clabconstants.Owner] = owner
	}

	return labels
}

// getclabBinaryPath determine the binary path of the running executable.
func getclabBinaryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	absPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

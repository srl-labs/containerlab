// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/spf13/cobra"
)

// toolsCmd represents the tools command
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "various tools your lab might need",
	Long:  "tools command groups various tools you might need for your lab\nreference: https://containerlab.dev/cmd/tools/",
}

func init() {
	rootCmd.AddCommand(toolsCmd)
}

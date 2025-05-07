// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	toolsCmd.AddCommand(apiServerCmd)
}

// apiServerCmd represents the api-server command container
var apiServerCmd = &cobra.Command{
	Use:   "api-server",
	Short: "Containerlab API server operations",
	Long:  "Start, stop, and manage Containerlab API server containers",
}

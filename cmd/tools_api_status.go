// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

// APIServerListItem defines the structure for API server container info in JSON output
type APIServerListItem struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	LabsDir string `json:"labs_dir"`
	Runtime string `json:"runtime"`
	Owner   string `json:"owner"`
}

func init() {
	apiServerCmd.AddCommand(apiServerStatusCmd)
	apiServerStatusCmd.Flags().StringVarP(&outputFormatAPI, "format", "f", "table",
		"output format for 'status' command (table, json)")
}

// apiServerStatusCmd shows status of active API server containers
var apiServerStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "show status of active Containerlab API server containers",
	PreRunE: common.CheckAndGetRootPrivs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use common.Runtime for consistency with other commands
		runtimeName := common.Runtime
		if runtimeName == "" {
			runtimeName = apiServerRuntime
		}

		// Initialize containerlab with runtime using the same approach as inspect command
		opts := []clab.ClabOption{
			clab.WithTimeout(common.Timeout),
			clab.WithRuntime(runtimeName,
				&runtime.RuntimeConfig{
					Debug:            common.Debug,
					Timeout:          common.Timeout,
					GracefulShutdown: common.Graceful,
				},
			),
			clab.WithDebug(common.Debug),
		}

		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		// Check connectivity like inspect does
		err = c.CheckConnectivity(ctx)
		if err != nil {
			return err
		}

		// Filter only by API server label
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "tool-type",
				Operator:   "=",
				Match:      "api-server",
			},
		}

		containers, err := c.ListContainers(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) == 0 {
			if outputFormatAPI == "json" {
				fmt.Println("[]")
			} else {
				fmt.Println("No active API server containers found")
			}
			return nil
		}

		// Process containers and format output
		listItems := make([]APIServerListItem, 0, len(containers))
		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")

			// Get port from labels or use default
			port := 8080 // default
			if portStr, ok := c.Labels["clab-api-port"]; ok {
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				}
			}

			// Get host from labels or use default
			host := "localhost" // default
			if hostVal, ok := c.Labels["clab-api-host"]; ok {
				host = hostVal
			}

			// Get labs dir from labels or use default
			labsDir := "~/.clab" // default
			if dirsVal, ok := c.Labels["clab-labs-dir"]; ok {
				labsDir = dirsVal
			}

			// Get runtime from labels or use default
			runtimeType := "docker" // default
			if rtVal, ok := c.Labels["clab-runtime"]; ok {
				runtimeType = rtVal
			}

			// Get owner from container labels
			owner := "N/A"
			if ownerVal, exists := c.Labels[clabels.Owner]; exists && ownerVal != "" {
				owner = ownerVal
			}

			listItems = append(listItems, APIServerListItem{
				Name:    name,
				State:   c.State,
				Host:    host,
				Port:    port,
				LabsDir: labsDir,
				Runtime: runtimeType,
				Owner:   owner,
			})
		}

		// Output based on format
		if outputFormatAPI == "json" {
			b, err := json.MarshalIndent(listItems, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal to JSON: %w", err)
			}
			fmt.Println(string(b))
		} else {
			// Use go-pretty table
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleRounded)
			t.Style().Format.Header = text.FormatTitle
			t.Style().Options.SeparateRows = true

			t.AppendHeader(table.Row{"NAME", "STATUS", "HOST", "PORT", "LABS DIR", "RUNTIME", "OWNER"})

			for _, item := range listItems {
				t.AppendRow(table.Row{
					item.Name,
					item.State,
					item.Host,
					item.Port,
					item.LabsDir,
					item.Runtime,
					item.Owner,
				})
			}
			t.Render()
		}

		return nil
	},
}

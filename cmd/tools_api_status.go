// Copyright 2025
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
)

// APIServerListItem defines the structure for API server container info in JSON output.
type APIServerListItem struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	LabsDir string `json:"labs_dir"`
	Runtime string `json:"runtime"`
	Owner   string `json:"owner"`
}

func apiServerStatus(cobraCmd *cobra.Command, o *Options) error {
	ctx := cobraCmd.Context()

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	// Check connectivity like inspect does
	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	containers, err := c.ListContainers(ctx, clabcore.WithListToolType("api-server"))
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		if o.ToolsAPI.OutputFormat == clabconstants.FormatJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No active API server containers found")
		}

		return nil
	}

	// Process containers and format output
	listItems := make([]APIServerListItem, 0, len(containers))
	for idx := range containers {
		name := strings.TrimPrefix(containers[idx].Names[0], "/")

		// Get port from labels or use default
		port := 8080 // default

		if portStr, ok := containers[idx].Labels["clab-api-port"]; ok {
			if portVal, err := strconv.Atoi(portStr); err == nil {
				port = portVal
			}
		}

		// Get host from labels or use default
		host := "localhost" // default
		if hostVal, ok := containers[idx].Labels["clab-api-host"]; ok {
			host = hostVal
		}

		// Get labs dir from labels or use default
		labsDir := "~/.clab" // default
		if dirsVal, ok := containers[idx].Labels["clab-labs-dir"]; ok {
			labsDir = dirsVal
		}

		// Get runtime from labels or use default
		runtimeType := "docker" // default
		if rtVal, ok := containers[idx].Labels["clab-runtime"]; ok {
			runtimeType = rtVal
		}

		// Get owner from container labels
		owner := clabconstants.NotApplicable

		ownerVal, exists := containers[idx].Labels[clabconstants.Owner]
		if exists && ownerVal != "" {
			owner = ownerVal
		}

		listItems = append(listItems, APIServerListItem{
			Name:    name,
			State:   containers[idx].State,
			Host:    host,
			Port:    port,
			LabsDir: labsDir,
			Runtime: runtimeType,
			Owner:   owner,
		})
	}

	if o.ToolsAPI.OutputFormat == clabconstants.FormatJSON {
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
}

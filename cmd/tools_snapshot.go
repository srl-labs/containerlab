// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	defaultSnapshotTimeout = 5 * time.Minute
	snapshotPollInterval   = 2 * time.Second
)

func snapshotCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "snapshot operations for vrnetlab-based nodes",
		Long: "snapshot command provides operations to save and manage VM snapshots " +
			"for vrnetlab-based nodes in your lab",
	}

	saveCmd := &cobra.Command{
		Use:   "save",
		Short: "save VM snapshots from running nodes",
		Long: "save creates snapshots of running vrnetlab-based VMs and saves them to disk.\n" +
			"Each node's snapshot is saved as {output-dir}/{nodename}.tar\n" +
			"Non-vrnetlab nodes are automatically skipped.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return snapshotSaveFn(cmd, o)
		},
	}

	saveCmd.Flags().StringVar(
		&o.ToolsSnapshot.OutputDir,
		"output-dir",
		o.ToolsSnapshot.OutputDir,
		"directory to save snapshot files (creates {dir}/{node}.tar)",
	)

	saveCmd.Flags().StringSliceVar(
		&o.Filter.NodeFilter,
		"node-filter",
		o.Filter.NodeFilter,
		"comma separated list of nodes to snapshot",
	)

	saveCmd.Flags().StringVar(
		&o.ToolsSnapshot.Format,
		"format",
		o.ToolsSnapshot.Format,
		"output format: table, json",
	)

	saveCmd.Flags().DurationVar(
		&o.ToolsSnapshot.Timeout,
		"timeout",
		o.ToolsSnapshot.Timeout,
		"timeout per node for snapshot creation",
	)

	c.AddCommand(saveCmd)
	return c, nil
}

func snapshotSaveFn(cmd *cobra.Command, o *Options) error {
	ctx := cmd.Context()

	// Initialize CLab
	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	// Create output directory
	if err := os.MkdirAll(o.ToolsSnapshot.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get containers to snapshot (respects node-filter)
	containers, err := c.ListNodesContainers(ctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found matching filters")
	}

	// Create snapshot collection for tracking results
	collection := NewSnapshotCollection()

	var wg sync.WaitGroup

	log.Infof("Creating snapshots for %d node(s)...", len(containers))

	// Process each container concurrently
	for idx := range containers {
		wg.Add(1)
		go func(container clabruntime.GenericContainer) {
			defer wg.Done()

			nodeName := container.Labels[clabconstants.NodeName]
			outputPath := filepath.Join(o.ToolsSnapshot.OutputDir, nodeName+".tar")

			result := createNodeSnapshot(ctx, container, outputPath, o.ToolsSnapshot.Timeout)
			collection.Add(result)

			// Log result
			if result.Error != nil {
				log.Errorf("%s: %v", nodeName, result.Error)
			} else if result.Status == "skipped" {
				log.Infof("%s: skipping (%s)", nodeName, result.Reason)
			} else {
				log.Info("snapshot saved",
					"node",
					nodeName,
					"path", result.SnapshotPath,
					"size", formatBytes(result.SizeBytes),
					"duration", result.Duration.Round(time.Second))
			}
		}(containers[idx])
	}

	wg.Wait()

	// Print summary
	log.Infof("Summary: %s", collection.Summary())

	return nil
}

// SnapshotResult represents the result of a snapshot operation for one node.
type SnapshotResult struct {
	NodeName     string
	Status       string // "success", "failed", "skipped"
	SnapshotPath string
	SizeBytes    int64
	Duration     time.Duration
	Error        error
	Reason       string
}

// SnapshotCollection aggregates results from multiple node snapshot operations.
type SnapshotCollection struct {
	results map[string]*SnapshotResult
	m       sync.RWMutex
}

// NewSnapshotCollection creates a new snapshot collection.
func NewSnapshotCollection() *SnapshotCollection {
	return &SnapshotCollection{
		results: make(map[string]*SnapshotResult),
	}
}

// Add adds a snapshot result to the collection.
func (sc *SnapshotCollection) Add(result *SnapshotResult) {
	sc.m.Lock()
	defer sc.m.Unlock()
	sc.results[result.NodeName] = result
}

// Summary returns a summary string of the snapshot operations.
func (sc *SnapshotCollection) Summary() string {
	sc.m.RLock()
	defer sc.m.RUnlock()

	var success, failed, skipped int
	var totalSize int64

	for _, r := range sc.results {
		switch r.Status {
		case "success":
			success++
			totalSize += r.SizeBytes
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}

	return fmt.Sprintf("%d succeeded, %d failed, %d skipped (%s total)",
		success, failed, skipped, formatBytes(totalSize))
}

// createNodeSnapshot creates a snapshot for a single node.
func createNodeSnapshot(ctx context.Context, container clabruntime.GenericContainer,
	outputPath string, timeout time.Duration,
) *SnapshotResult {
	start := time.Now()
	result := &SnapshotResult{
		NodeName:     container.Labels[clabconstants.NodeName],
		SnapshotPath: outputPath,
	}

	// Check if this is a vrnetlab node
	if !isVrnetlabNode(container) {
		kind := container.Labels[clabconstants.NodeKind]
		result.Status = "skipped"
		result.Reason = fmt.Sprintf(
			"not a vrnetlab node (kind=%s, image=%s)",
			kind,
			container.Image,
		)
		return result
	}

	containerName := container.Names[0]
	runtime := container.Runtime

	// 1. Trigger snapshot creation
	log.Debugf("%s: triggering snapshot creation", result.NodeName)
	execCmd := clabexec.NewExecCmdFromSlice([]string{"touch", "/snapshot-save"})
	if err := runtime.ExecNotWait(ctx, containerName, execCmd); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("failed to trigger snapshot: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// 2. Wait for snapshot to complete
	log.Debugf("%s: waiting for snapshot completion", result.NodeName)
	if err := waitForSnapshotFile(ctx, runtime, containerName, timeout); err != nil {
		result.Status = "failed"
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// 3. Copy snapshot from container to host
	log.Debugf("%s: copying snapshot to host", result.NodeName)
	if err := copySnapshotFromContainer(ctx, containerName, outputPath); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("failed to copy snapshot: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Get file size
	if fi, err := os.Stat(outputPath); err == nil {
		result.SizeBytes = fi.Size()
	}

	result.Status = "success"
	result.Duration = time.Since(start)
	return result
}

// waitForSnapshotFile waits for vrnetlab to complete snapshot creation by monitoring logs.
// Vrnetlab logs "Snapshot saved to /snapshot.tar" when the snapshot is complete.
func waitForSnapshotFile(ctx context.Context, runtime clabruntime.ContainerRuntime,
	containerName string, timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	// Get log stream from container
	logReader, err := runtime.StreamLogs(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to stream logs: %w", err)
	}
	defer logReader.Close()

	// Create a buffered reader to read logs line by line
	logScanner := make(chan string, 100)
	errChan := make(chan error, 1)

	// Start goroutine to read logs
	go func() {
		defer close(logScanner)
		buf := make([]byte, 4096)
		var partial string

		for {
			n, err := logReader.Read(buf)
			if n > 0 {
				// Combine with any partial line from previous read
				text := partial + string(buf[:n])
				lines := strings.Split(text, "\n")

				// Last element might be incomplete
				partial = lines[len(lines)-1]

				// Send complete lines
				for i := 0; i < len(lines)-1; i++ {
					select {
					case logScanner <- lines[i]:
					case <-ctx.Done():
						return
					}
				}
			}
			if err != nil {
				if err.Error() != "EOF" {
					errChan <- err
				}
				return
			}
		}
	}()

	// Wait for snapshot completion message
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errChan:
			return fmt.Errorf("error reading logs: %w", err)

		case line, ok := <-logScanner:
			if !ok {
				return fmt.Errorf("log stream closed before snapshot completed")
			}

			// Check for failure message first
			if strings.Contains(line, "Snapshot save failed:") {
				return fmt.Errorf("snapshot save failed: %s", line)
			}

			// Check for completion message (vrnetlab now saves to /snapshot-output.tar)
			if strings.Contains(line, "Snapshot saved to /snapshot-output.tar") {
				log.Debugf("%s: snapshot creation complete", containerName)
				return nil
			}

			// Also log any errors from vrnetlab
			if strings.Contains(line, "ERROR") || strings.Contains(line, "Error") {
				log.Debugf("%s: %s", containerName, line)
			}

		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for snapshot after %v", timeout)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for snapshot after %v", timeout)
		}
	}
}

// copySnapshotFromContainer copies the snapshot file from container to host.
func copySnapshotFromContainer(ctx context.Context, containerName, outputPath string) error {
	// Use docker cp command to copy snapshot from container
	// vrnetlab saves to /snapshot-output.tar when triggered by /snapshot-save
	cmd := exec.CommandContext(ctx, "docker", "cp",
		containerName+":/snapshot-output.tar",
		outputPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp failed: %w, output: %s", err, string(output))
	}

	return nil
}

// isVrnetlabKind checks if a node is a vrnetlab-based node.
// It checks if the node's image starts with "vrnetlab/".
func isVrnetlabNode(container clabruntime.GenericContainer) bool {
	// Check if image starts with vrnetlab/
	return strings.HasPrefix(container.Image, "vrnetlab/")
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/clab"
	clabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/runtime"
)

// createLabels creates container labels
func CreateLabels(labName, containerName, owner, toolType string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		"containerlab":       labName,
		"clab-node-name":     shortName,
		"clab-node-longname": containerName,
		"clab-node-kind":     "linux",
		"clab-node-group":    "",
		"clab-node-type":     "tool",
		"tool-type":          toolType,
	}

	// Add topology file path
	if Topo != "" {
		absPath, err := filepath.Abs(Topo)
		if err == nil {
			labels["clab-topo-file"] = absPath
		} else {
			labels["clab-topo-file"] = Topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(Topo)
		labels["clab-node-lab-dir"] = filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[clabels.Owner] = owner
	}

	return labels
}

// GetLabConfig gets lab configuration and returns lab name, network name and containerlab instance
func GetLabConfig(ctx context.Context, labName string) (string, string, *clab.CLab, error) {
	var c *clab.CLab
	var err error

	// If no topology path or lab name provided, use current directory as topo path
	if Topo == "" && labName == "" {
		cwd, err := os.Getwd()
		if err == nil {
			Topo = cwd
		}
	}

	// Create a single containerlab instance
	opts := []clab.ClabOption{
		clab.WithTimeout(Timeout),
		clab.WithRuntime(Runtime, &runtime.RuntimeConfig{
			Debug:            Debug,
			Timeout:          Timeout,
			GracefulShutdown: Graceful,
		}),
		clab.WithDebug(Debug),
	}

	if Topo != "" {
		opts = append(opts, clab.WithTopoPath(Topo, VarsFile))
	} else if labName != "" {
		opts = append(opts, clab.WithTopoFromLab(labName))
	} else {
		return "", "", nil, fmt.Errorf("no topology file found or provided")
	}

	c, err = clab.NewContainerLab(opts...)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create containerlab instance: %w", err)
	}

	// update Topo with the absolute topology file path
	if c.TopoPaths != nil && c.TopoPaths.TopologyFileIsSet() {
		Topo = c.TopoPaths.TopologyFilenameAbsPath()
		log.Debugf("Using topology file: %s", Topo)
	}

	if c.Config == nil {
		return "", "", nil, fmt.Errorf("failed to load lab configuration")
	}

	// Get lab name if not provided
	if labName == "" {
		labName = c.Config.Name
	}

	// Get network name
	networkName := c.Config.Mgmt.Network
	if networkName == "" {
		networkName = "clab-" + c.Config.Name
	}

	return labName, networkName, c, nil
}

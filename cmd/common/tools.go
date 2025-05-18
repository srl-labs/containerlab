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
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
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

// GetOwner determines the owner name from a provided parameter or environment variables.
// It first checks the provided owner parameter, then falls back to SUDO_USER environment
// variable, and finally the USER environment variable.
// GetOwner determines the owner name from a provided parameter or environment variables.
// It first checks the provided owner parameter, then falls back to SUDO_USER environment
// variable, and finally the USER environment variable. This function is now located in
// the utils package and kept here for backward compatibility.
// TODO: remove this wrapper once all callers are migrated to utils.GetOwner.
func GetOwner(owner string) string {
	return utils.GetOwner(owner)
}

// GetLabConfig gets lab configuration and returns lab name, network name and containerlab instance
func GetLabConfig(ctx context.Context, labName string) (string, string, *clab.CLab, error) {
	var c *clab.CLab
	var err error

	// If topo file is provided or discovered
	if Topo == "" && labName == "" {
		cwd, err := os.Getwd()
		if err == nil {
			Topo, err = clab.FindTopoFileByPath(cwd)
			if err == nil {
				log.Debugf("Found topology file: %s", Topo)
			}
		}
	}

	// If we have lab name but no topo file, try to find it from containers
	if labName != "" && Topo == "" {
		_, rinit, err := clab.RuntimeInitializer(Runtime)
		if err != nil {
			return "", "", nil, err
		}

		rt := rinit()
		err = rt.Init(runtime.WithConfig(&runtime.RuntimeConfig{Timeout: Timeout}))
		if err != nil {
			return "", "", nil, err
		}

		// Find containers for this lab
		filter := []*types.GenericFilter{
			{
				FilterType: "label",
				Field:      "containerlab",
				Operator:   "=",
				Match:      labName,
			},
		}
		containers, err := rt.ListContainers(ctx, filter)
		if err != nil {
			return "", "", nil, err
		}

		if len(containers) == 0 {
			return "", "", nil, fmt.Errorf("lab '%s' not found - no running containers", labName)
		}

		// Get topo file from container labels
		topoFile := containers[0].Labels["clab-topo-file"]
		if topoFile == "" {
			return "", "", nil, fmt.Errorf("could not determine topology file from container labels")
		}

		log.Debugf("Found topology file for lab %s: %s", labName, topoFile)
		Topo = topoFile
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
	} else {
		return "", "", nil, fmt.Errorf("no topology file found or provided")
	}

	c, err = clab.NewContainerLab(opts...)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create containerlab instance: %w", err)
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

package common

import (
	"path/filepath"
	"strings"

	clabels "github.com/srl-labs/containerlab/labels"
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

package common

import (
	"path/filepath"
	"strings"

	clabels "github.com/srl-labs/containerlab/labels"
)

// CreateLabelsMap creates container labels map for additional containers launched after the lab is up. Such as sshx, gotty, etc.
func CreateLabelsMap(labName, containerName, owner, toolType string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		clabels.Containerlab: labName,
		clabels.NodeName:     shortName,
		clabels.LongName:     containerName,
		clabels.NodeKind:     "linux",
		clabels.NodeGroup:    "",
		clabels.NodeType:     "tool",
		clabels.ToolType:     toolType,
	}

	// Add topology file path
	if Topo != "" {
		absPath, err := filepath.Abs(Topo)
		if err == nil {
			labels[clabels.TopoFile] = absPath
		} else {
			labels[clabels.TopoFile] = Topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(Topo)
		labels[clabels.NodeLabDir] = filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[clabels.Owner] = owner
	}

	return labels
}

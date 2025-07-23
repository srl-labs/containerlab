package common

import (
	"path/filepath"
	"strings"

	containerlablabels "github.com/srl-labs/containerlab/labels"
)

// CreateLabelsMap creates container labels map for additional containers launched after the lab is up. Such as sshx, gotty, etc.
func CreateLabelsMap(labName, containerName, owner, toolType string) map[string]string {
	shortName := strings.Replace(containerName, "clab-"+labName+"-", "", 1)

	labels := map[string]string{
		containerlablabels.Containerlab: labName,
		containerlablabels.NodeName:     shortName,
		containerlablabels.LongName:     containerName,
		containerlablabels.NodeKind:     "linux",
		containerlablabels.NodeGroup:    "",
		containerlablabels.NodeType:     "tool",
		containerlablabels.ToolType:     toolType,
	}

	// Add topology file path
	if Topo != "" {
		absPath, err := filepath.Abs(Topo)
		if err == nil {
			labels[containerlablabels.TopoFile] = absPath
		} else {
			labels[containerlablabels.TopoFile] = Topo
		}

		// Set node lab directory
		baseDir := filepath.Dir(Topo)
		labels[containerlablabels.NodeLabDir] =
			filepath.Join(baseDir, "clab-"+labName, shortName)
	}

	// Add owner label if available
	if owner != "" {
		labels[containerlablabels.Owner] = owner
	}

	return labels
}

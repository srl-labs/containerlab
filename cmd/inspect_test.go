package cmd

import (
	"slices"
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func TestLogicalNodeContainers(t *testing.T) {
	ctr := func(lab, name, root string, internal bool, ip string) clabruntime.GenericContainer {
		labels := map[string]string{
			clabconstants.Containerlab: lab,
			clabconstants.NodeName:     name,
		}
		if root != "" {
			labels[clabconstants.RootNodeName] = root
		}

		if internal {
			labels[clabconstants.InternalNode] = "true"
		}

		return clabruntime.GenericContainer{
			Names:           []string{"clab-" + lab + "-" + name},
			Labels:          labels,
			NetworkSettings: clabruntime.GenericMgmtIPs{IPv4addr: ip},
		}
	}

	tool := ctr("lab1", "", "", false, "10.0.0.9")
	tool.Labels[clabconstants.ToolType] = "sshx"

	got := logicalNodeContainers([]clabruntime.GenericContainer{
		tool,
		// internal node without a root node.
		ctr("lab1", "tailscale", "", true, "10.0.0.8"),
		ctr("lab1", "linux", "", false, "10.0.0.1"),
		// SR Linux node with a netns holder.
		ctr("lab1", "srl", "", false, ""),
		ctr("lab1", "srl-netns", "srl", true, "10.0.0.2"),
		// distributed SR-SIM node.
		ctr("lab1", "sr-1", "sr", false, ""),
		ctr("lab1", "sr-b", "sr", false, ""),
		ctr("lab1", "sr-a", "sr", false, ""),
		ctr("lab1", "sr-netns", "sr", true, "10.0.0.3"),
		// distributed SR-SIM node without CPM A, and a node name shared with lab1.
		ctr("lab2", "sr-1", "sr", false, ""),
		ctr("lab2", "sr-b", "sr", false, ""),
		ctr("lab2", "sr-netns", "sr", true, "10.0.0.4"),
		ctr("lab2", "linux", "", false, "10.0.0.5"),
	})

	want := []string{
		"clab-lab1-linux 10.0.0.1",
		"clab-lab1-srl 10.0.0.2",
		"clab-lab1-sr-a 10.0.0.3",
		"clab-lab2-sr-b 10.0.0.4",
		"clab-lab2-linux 10.0.0.5",
	}

	gotRows := make([]string, 0, len(got))
	for idx := range got {
		gotRows = append(gotRows, got[idx].Names[0]+" "+got[idx].NetworkSettings.IPv4addr)
	}

	if !slices.Equal(gotRows, want) {
		t.Fatalf("logical node containers mismatch\n got: %v\nwant: %v", gotRows, want)
	}
}

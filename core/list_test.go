package core

import (
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestFoldInternalContainers(t *testing.T) {
	containers := []clabruntime.GenericContainer{
		{
			Names:  []string{"clab-lab1-srl1"},
			Labels: map[string]string{clabconstants.LongName: "clab-lab1-srl1"},
		},
		{
			Names: []string{"clab-lab1-srl1-netns"},
			Labels: map[string]string{
				clabconstants.LongName:         "clab-lab1-srl1-netns",
				clabconstants.InternalNode:     "true",
				clabconstants.RootNodeLongName: "clab-lab1-srl1",
			},
			NetworkName:     "clab",
			NetworkSettings: clabruntime.GenericMgmtIPs{IPv4addr: "172.20.20.2", IPv4pLen: 24},
			Ports:           []*clabtypes.GenericPortBinding{{HostPort: 2222}},
		},
		{
			Names:           []string{"clab-lab1-client1"},
			Labels:          map[string]string{clabconstants.LongName: "clab-lab1-client1"},
			NetworkSettings: clabruntime.GenericMgmtIPs{IPv4addr: "172.20.20.3", IPv4pLen: 24},
		},
	}

	got := FoldInternalContainers(containers)
	if len(got) != 2 {
		t.Fatalf("expected internal container to be removed, got %d containers", len(got))
	}

	srl := got[0]
	if srl.Names[0] != "clab-lab1-srl1" {
		t.Fatalf("unexpected first container %q", srl.Names[0])
	}

	if srl.NetworkName != "clab" || srl.NetworkSettings.IPv4addr != "172.20.20.2" {
		t.Errorf("holder network settings not copied: %q %+v", srl.NetworkName, srl.NetworkSettings)
	}

	if len(srl.Ports) != 1 || srl.Ports[0].HostPort != 2222 {
		t.Errorf("holder ports not copied: %+v", srl.Ports)
	}

	if len(containers) != 3 || containers[0].NetworkName != "" {
		t.Errorf("input slice was modified: %+v", containers)
	}

	if got[1].NetworkSettings.IPv4addr != "172.20.20.3" {
		t.Errorf("unrelated container changed: %+v", got[1].NetworkSettings)
	}
}

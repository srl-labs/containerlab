//go:build linux && podman

package podman

import (
	"context"
	"testing"

	"github.com/srl-labs/containerlab/types"
)

func TestCreateContainerSpecPreservesImageCommandDefaults(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:  "clab-test-node1",
		ShortName: "node1",
		Image:     "localhost/test:latest",
		Labels:    map[string]string{},
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.Command != nil {
		t.Fatalf("Command = %#v, want nil to preserve the image default", sg.Command)
	}
	if sg.Entrypoint != nil {
		t.Fatalf("Entrypoint = %#v, want nil to preserve the image default", sg.Entrypoint)
	}
}

//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"testing"

	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/srl-labs/containerlab/types"
)

func TestCreateContainerSpecAppliesRuntimeNamespaceAndTmpfs(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:     "clab-test-node1",
		ShortName:    "node1",
		Image:        "localhost/test:latest",
		Labels:       map[string]string{},
		NetworkMode:  "host",
		CgroupnsMode: "host",
		ShmSize:      "64m",
		Tmpfs:        map[string]string{"/run": "rw,nosuid,nodev", "/run/lock": "rw"},
		ExtraHosts:   []string{"example:127.0.0.1"},
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.CgroupNS.NSMode != specgen.Host {
		t.Fatalf("CgroupNS mode = %q, want %q", sg.CgroupNS.NSMode, specgen.Host)
	}
	if sg.ShmSize == nil || *sg.ShmSize != 64*1000*1000 {
		t.Fatalf("ShmSize = %v, want 64000000", sg.ShmSize)
	}

	tmpfs := map[string][]string{}
	for _, mount := range sg.Mounts {
		if mount.Type == "tmpfs" {
			tmpfs[mount.Destination] = mount.Options
		}
	}
	if _, ok := tmpfs["/run"]; !ok {
		t.Fatalf("tmpfs mounts = %#v, missing /run", tmpfs)
	}
	if _, ok := tmpfs["/run/lock"]; !ok {
		t.Fatalf("tmpfs mounts = %#v, missing /run/lock", tmpfs)
	}
}

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

func TestCreateContainerSpecAppliesConfiguredHostname(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:  "clab-test-node1",
		ShortName: "node1",
		Hostname:  "dns-private-master-01001",
		Image:     "localhost/test:latest",
		Labels:    map[string]string{},
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.Hostname != cfg.Hostname {
		t.Fatalf("Hostname = %q, want %q", sg.Hostname, cfg.Hostname)
	}
}

func TestCreateContainerSpecAppliesHostnameWithContainerNetworkMode(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:    "clab-test-child",
		ShortName:   "child",
		Hostname:    "dns-private-master-01001",
		Image:       "localhost/test:latest",
		Labels:      map[string]string{},
		NetworkMode: "container:provider",
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.Hostname != "" {
		t.Fatalf("Hostname = %q, want empty so the provider owns the hostname", sg.Hostname)
	}
	if sg.UtsNS.NSMode != specgen.FromContainer {
		t.Fatalf("UtsNS mode = %q, want %q", sg.UtsNS.NSMode, specgen.FromContainer)
	}
	if sg.UtsNS.Value != "clab-test-provider" {
		t.Fatalf("UtsNS value = %q, want clab-test-provider", sg.UtsNS.Value)
	}
	if sg.NetNS.NSMode != specgen.FromContainer {
		t.Fatalf("NetNS mode = %q, want %q", sg.NetNS.NSMode, specgen.FromContainer)
	}
	if sg.NetNS.Value != "clab-test-provider" {
		t.Fatalf("NetNS value = %q, want clab-test-provider", sg.NetNS.Value)
	}
}

func TestContainerModeProviderName(t *testing.T) {
	tests := []struct {
		name        string
		networkMode string
		want        string
	}{
		{
			name:        "short provider name",
			networkMode: "container:provider",
			want:        "clab-test-provider",
		},
		{
			name:        "long provider name",
			networkMode: "container:clab-test-provider",
			want:        "clab-test-provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &types.NodeConfig{
				LongName:    "clab-test-child",
				ShortName:   "child",
				NetworkMode: tt.networkMode,
			}

			got, err := containerModeProviderName(cfg)
			if err != nil {
				t.Fatalf("containerModeProviderName returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("containerModeProviderName = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateContainerSpecDefaultsHostnameToNodeName(t *testing.T) {
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

	if sg.Hostname != cfg.ShortName {
		t.Fatalf("Hostname = %q, want %q", sg.Hostname, cfg.ShortName)
	}
}

func TestCreateContainerSpecAppliesNoneNetworkMode(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:    "clab-test-node1",
		ShortName:   "node1",
		Image:       "localhost/test:latest",
		Labels:      map[string]string{},
		NetworkMode: "none",
		ExtraHosts:  []string{"example:127.0.0.1"},
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.NetNS.NSMode != specgen.NoNetwork {
		t.Fatalf("NetNS mode = %q, want %q", sg.NetNS.NSMode, specgen.NoNetwork)
	}
}

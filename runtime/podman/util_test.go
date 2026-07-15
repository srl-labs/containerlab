//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"strings"
	"testing"

	"github.com/srl-labs/containerlab/types"
	"go.podman.io/podman/v6/pkg/specgen"
)

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

	if sg.Hostname != cfg.Hostname {
		t.Fatalf("Hostname = %q, want %q", sg.Hostname, cfg.Hostname)
	}
	if sg.UtsNS.NSMode != specgen.Private {
		t.Fatalf("UtsNS mode = %q, want %q", sg.UtsNS.NSMode, specgen.Private)
	}
	if sg.NetNS.NSMode != "container" {
		t.Fatalf("NetNS mode = %q, want container", sg.NetNS.NSMode)
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

func TestCreateContainerSpecAppliesRuntimeNamespaceAndTmpfs(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	cfg := &types.NodeConfig{
		LongName:     "clab-test-node1",
		ShortName:    "node1",
		Image:        "localhost/test:latest",
		Labels:       map[string]string{},
		NetworkMode:  "host",
		CgroupnsMode: "host",
		CgroupParent: "/xform/my-lab/leaves",
		ShmSize:      "64m",
		Tmpfs:        map[string]string{"/run": "rw,nosuid,nodev", "/run/lock": "rw"},
		Devices:      []string{"/dev/null"},
		ExtraHosts:   []string{"example:127.0.0.1"},
	}

	sg, err := r.createContainerSpec(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createContainerSpec returned error: %v", err)
	}

	if sg.CgroupNS.NSMode != specgen.Host {
		t.Fatalf("CgroupNS mode = %q, want %q", sg.CgroupNS.NSMode, specgen.Host)
	}
	if sg.CgroupParent != "/xform/my-lab/leaves" {
		t.Fatalf("CgroupParent = %q, want /xform/my-lab/leaves", sg.CgroupParent)
	}
	if sg.ShmSize == nil || *sg.ShmSize != 64*1000*1000 {
		t.Fatalf("ShmSize = %v, want 64000000", sg.ShmSize)
	}
	if len(sg.Devices) != 1 || sg.Devices[0].Path != "/dev/null" {
		t.Fatalf("Devices = %#v, want /dev/null", sg.Devices)
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

func TestConvertMountsSeparatesManagedVolumes(t *testing.T) {
	mounts, volumes, err := (&PodmanRuntime{}).convertMounts(
		context.Background(),
		[]string{"/host:/etc:ro"},
		[]string{"shared:/shared:ro,nocopy", "/cache"},
	)
	if err != nil {
		t.Fatalf("convertMounts() unexpected error: %v", err)
	}
	if len(mounts) != 1 {
		t.Fatalf("convertMounts() returned %d bind mounts, want 1", len(mounts))
	}
	if mounts[0].Type != "bind" || mounts[0].Source != "/host" || mounts[0].Destination != "/etc" {
		t.Fatalf("bind mount = %+v, want host bind mount", mounts[0])
	}
	if len(volumes) != 2 {
		t.Fatalf("convertMounts() returned %d managed volumes, want 2", len(volumes))
	}

	if volumes[0].Name != "shared" ||
		volumes[0].Dest != "/shared" ||
		volumes[0].IsAnonymous ||
		len(volumes[0].Options) != 2 ||
		volumes[0].Options[0] != "ro" ||
		volumes[0].Options[1] != "nocopy" {
		t.Fatalf("named volume = %+v, want named read-only no-copy volume", volumes[0])
	}
	if volumes[1].Name != "" || volumes[1].Dest != "/cache" || !volumes[1].IsAnonymous {
		t.Fatalf("anonymous volume = %+v, want anonymous managed volume", volumes[1])
	}
}

func TestCreateContainerSpecUsesNamedVolumes(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	sg, err := r.createContainerSpec(context.Background(), &types.NodeConfig{
		LongName: "clab-test-node1",
		Image:    "localhost/test:latest",
		Labels:   map[string]string{},
		Binds:    []string{"/host:/etc"},
		Volumes:  []string{"shared:/shared"},
	})
	if err != nil {
		t.Fatalf("createContainerSpec() unexpected error: %v", err)
	}
	if len(sg.Mounts) != 1 || sg.Mounts[0].Type != "bind" {
		t.Fatalf("spec mounts = %+v, want one bind mount", sg.Mounts)
	}
	if len(sg.Volumes) != 1 ||
		sg.Volumes[0].Name != "shared" ||
		sg.Volumes[0].Dest != "/shared" {
		t.Fatalf("spec volumes = %+v, want one named volume", sg.Volumes)
	}
}

func TestCreateContainerSpecReturnsVolumeConversionError(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{Network: "clab"}}
	_, err := r.createContainerSpec(context.Background(), &types.NodeConfig{
		LongName: "clab-test-node1",
		Image:    "localhost/test:latest",
		Labels:   map[string]string{},
		Volumes:  []string{"shared:/shared:unsupported"},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to convert mounts") {
		t.Fatalf("createContainerSpec() error = %v, want volume conversion error", err)
	}
}

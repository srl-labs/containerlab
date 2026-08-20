package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestProcessCgroupnsMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		want    container.CgroupnsMode
		wantErr bool
	}{
		{name: "default", want: ""},
		{name: "host", mode: "host", want: "host"},
		{name: "private", mode: "private", want: "private"},
		{name: "invalid", mode: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostConfig := &container.HostConfig{}
			err := new(DockerRuntime).processCgroupnsMode(
				&clabtypes.NodeConfig{CgroupnsMode: tt.mode},
				hostConfig,
			)
			if (err != nil) != tt.wantErr {
				t.Fatalf("processCgroupnsMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && hostConfig.CgroupnsMode != tt.want {
				t.Fatalf("CgroupnsMode = %q, want %q", hostConfig.CgroupnsMode, tt.want)
			}
		})
	}
}

func TestProcessCgroupParent(t *testing.T) {
	tests := []struct {
		name   string
		parent string
	}{
		{name: "omitted"},
		{name: "configured", parent: "/xform/my-lab/leaves"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostConfig := &container.HostConfig{}
			new(DockerRuntime).processCgroupParent(
				&clabtypes.NodeConfig{CgroupParent: tt.parent},
				hostConfig,
			)
			if hostConfig.CgroupParent != tt.parent {
				t.Fatalf("CgroupParent = %q, want %q", hostConfig.CgroupParent, tt.parent)
			}
		})
	}
}

func TestCgroupParentCoexistsWithCgroupnsMode(t *testing.T) {
	node := &clabtypes.NodeConfig{
		CgroupnsMode: "host",
		CgroupParent: "/xform/my-lab/leaves",
	}
	hostConfig := &container.HostConfig{}

	if err := new(DockerRuntime).processCgroupnsMode(node, hostConfig); err != nil {
		t.Fatal(err)
	}
	new(DockerRuntime).processCgroupParent(node, hostConfig)

	if hostConfig.CgroupnsMode != "host" {
		t.Fatalf("CgroupnsMode = %q, want host", hostConfig.CgroupnsMode)
	}
	if hostConfig.CgroupParent != "/xform/my-lab/leaves" {
		t.Fatalf("CgroupParent = %q, want /xform/my-lab/leaves", hostConfig.CgroupParent)
	}
}

func TestConvertVolumeMounts(t *testing.T) {
	mounts, err := new(DockerRuntime).convertVolumeMounts([]string{
		"shared:/shared:ro,nocopy",
		"/cache",
	})
	if err != nil {
		t.Fatalf("convertVolumeMounts() unexpected error: %v", err)
	}
	if len(mounts) != 2 {
		t.Fatalf("convertVolumeMounts() returned %d mounts, want 2", len(mounts))
	}

	if mounts[0].Type != mount.TypeVolume ||
		mounts[0].Source != "shared" ||
		mounts[0].Target != "/shared" ||
		!mounts[0].ReadOnly ||
		mounts[0].VolumeOptions == nil ||
		!mounts[0].VolumeOptions.NoCopy {
		t.Fatalf("named volume mount = %+v, want read-only no-copy managed volume", mounts[0])
	}

	if mounts[1].Type != mount.TypeVolume ||
		mounts[1].Source != "" ||
		mounts[1].Target != "/cache" ||
		mounts[1].ReadOnly ||
		mounts[1].VolumeOptions != nil {
		t.Fatalf("anonymous volume mount = %+v, want managed anonymous volume", mounts[1])
	}
}

func TestConvertVolumeMountsRejectsUnsupportedOptions(t *testing.T) {
	_, err := new(DockerRuntime).convertVolumeMounts([]string{"shared:/shared:Z"})
	if err == nil || !strings.Contains(err.Error(), "unsupported volume option") {
		t.Fatalf("convertVolumeMounts() error = %v, want unsupported-option error", err)
	}
}

func TestCreateContainerReturnsVolumeConversionError(t *testing.T) {
	runtime := &DockerRuntime{
		config: clabruntime.RuntimeConfig{Timeout: time.Second},
	}

	_, err := runtime.CreateContainer(context.Background(), &clabtypes.NodeConfig{
		Volumes: []string{"shared:/shared:unsupported"},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to convert volume mounts") {
		t.Fatalf("CreateContainer() error = %v, want volume conversion error", err)
	}
}

package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
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

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

package docker

import (
	"testing"

	networkapi "github.com/docker/docker/api/types/network"
)

func TestMgmtBridgeNameFromInspect(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
		inspect     networkapi.Inspect
		podman      string
		want        string
	}{
		{
			name:        "podman netavark network_interface wins",
			networkName: "clab-mgmt",
			inspect: networkapi.Inspect{
				ID:      "cd38716161df05aff76bae83d9fc31415f843e2c2f939597307d4be0c1551fe6",
				Options: map[string]string{"com.docker.network.bridge.name": "br-cd38716161df"},
			},
			podman: "podman1",
			want:   "podman1",
		},
		{
			name:        "docker explicit bridge option",
			networkName: "clab-mgmt",
			inspect: networkapi.Inspect{
				ID:      "cd38716161df05aff76bae83d9fc31415f843e2c2f939597307d4be0c1551fe6",
				Options: map[string]string{"com.docker.network.bridge.name": "clab0"},
			},
			want: "clab0",
		},
		{
			name:        "docker default bridge",
			networkName: "bridge",
			inspect: networkapi.Inspect{
				ID: "cd38716161df05aff76bae83d9fc31415f843e2c2f939597307d4be0c1551fe6",
			},
			want: "docker0",
		},
		{
			name:        "docker generated bridge name",
			networkName: "clab-mgmt",
			inspect: networkapi.Inspect{
				ID: "cd38716161df05aff76bae83d9fc31415f843e2c2f939597307d4be0c1551fe6",
			},
			want: "br-cd38716161df",
		},
		{
			name:        "short id has no generated bridge",
			networkName: "clab-mgmt",
			inspect: networkapi.Inspect{
				ID: "short",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgmtBridgeNameFromInspect(tt.networkName, tt.inspect, tt.podman)
			if got != tt.want {
				t.Fatalf("got bridge name %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNetavarkNetworkInterface(t *testing.T) {
	got := netavarkNetworkInterface([]byte(`{"network_interface":"podman1"}`))
	if got != "podman1" {
		t.Fatalf("got network interface %q, want %q", got, "podman1")
	}
}

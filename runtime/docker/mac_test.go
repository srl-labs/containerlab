package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	networkapi "github.com/docker/docker/api/types/network"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestProcessNetworkModeSetsManagementMac(t *testing.T) {
	const macAddress = "aa:c1:ab:5b:9f:0e"

	runtime := &DockerRuntime{
		mgmt: &clabtypes.MgmtNet{Network: "clab"},
	}
	networking := new(networkapi.NetworkingConfig)

	err := runtime.processNetworkMode(
		context.Background(),
		networking,
		new(container.HostConfig),
		new(container.Config),
		&clabtypes.NodeConfig{MacAddress: macAddress},
	)
	if err != nil {
		t.Fatalf("processNetworkMode() failed: %v", err)
	}

	endpoint, ok := networking.EndpointsConfig["clab"]
	if !ok {
		t.Fatal("management network endpoint was not configured")
	}
	if endpoint.MacAddress != macAddress {
		t.Fatalf("management endpoint MAC = %q, want %q", endpoint.MacAddress, macAddress)
	}
}

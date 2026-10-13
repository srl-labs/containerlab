package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	networkapi "github.com/docker/docker/api/types/network"
	dockerC "github.com/docker/docker/client"
	"github.com/srl-labs/containerlab/mgmt"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestCreateContainerOnRuntimeAllocatedSubnet(t *testing.T) {
	const unsupported = "user specified IP address is supported only when connecting to networks with user configured subnets"

	for _, tc := range []struct {
		name       string
		ipv4       string
		ipv6       string
		failure    string
		retryError string
		wantCalls  int
		wantError  string
		existing   bool
		preferred  bool
		runtime    bool
		status     int
	}{
		{name: "automatic addresses", failure: unsupported, wantCalls: 2},
		{name: "existing addresses", failure: unsupported, existing: true, wantCalls: 2},
		{name: "preferred addresses", failure: unsupported, preferred: true, wantCalls: 2},
		{name: "user-configured subnet", wantCalls: 1},
		{name: "explicit IPv4", ipv4: "192.0.2.10", failure: unsupported, wantCalls: 2},
		{name: "explicit IPv6", ipv6: "2001:db8::10", failure: unsupported, wantCalls: 2},
		{
			name: "explicit addresses", ipv4: "192.0.2.10", ipv6: "2001:db8::10",
			failure: unsupported, wantCalls: 1, wantError: unsupported,
		},
		{
			name: "unrelated error", failure: "address already in use",
			wantCalls: 1, wantError: "address already in use",
		},
		{
			name: "retry failure", failure: unsupported, retryError: "permission denied",
			wantCalls: 2, wantError: "permission denied",
		},
		{
			name: "runtime provider", runtime: true, failure: unsupported,
			wantCalls: 1, wantError: unsupported,
		},
		{
			name: "server error", status: http.StatusInternalServerError, failure: unsupported,
			wantCalls: 1, wantError: unsupported,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &clabtypes.MgmtNet{
				Network: "reused", IPv4Subnet: "192.0.2.0/24", IPv6Subnet: "2001:db8::/64",
				IPAM: clabtypes.MgmtIPAM{Provider: clabtypes.IPAMProviderContainerlab},
			}
			node := &clabtypes.NodeConfig{
				ShortName: "r1", LongName: "clab-reused-r1", Image: "alpine:3",
				MgmtIPv4Address: tc.ipv4, MgmtIPv6Address: tc.ipv6,
			}
			var options clabtypes.AllocationOptions
			if tc.existing {
				options.Existing = []clabtypes.ExistingAddress{
					{NodeName: node.ShortName, Address: netip.MustParseAddr("192.0.2.20")},
					{NodeName: node.ShortName, Address: netip.MustParseAddr("2001:db8::20")},
				}
			}
			if tc.preferred {
				options.Preferred = map[string]clabtypes.NodeAddresses{
					node.ShortName: {IPv4: "192.0.2.30", IPv6: "2001:db8::30"},
				}
			}
			if tc.runtime {
				m.IPAM.Provider = clabtypes.IPAMProviderRuntime
			}
			if err := mgmt.AllocateManagementIPs(context.Background(), m,
				[]*clabtypes.NodeConfig{node}, options); err != nil {
				t.Fatal(err)
			}
			first4, first6 := node.MgmtIPv4Address, node.MgmtIPv6Address
			var requests []networkapi.EndpointIPAMConfig
			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost ||
						!strings.HasSuffix(r.URL.Path, "/containers/create") {
						t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
						http.NotFound(w, r)
						return
					}
					var request struct {
						NetworkingConfig networkapi.NetworkingConfig
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					endpoint := request.NetworkingConfig.EndpointsConfig[m.Network]
					if endpoint == nil || endpoint.IPAMConfig == nil {
						t.Error("missing management endpoint IPAM configuration")
						return
					}
					requests = append(requests, *endpoint.IPAMConfig)
					failure := tc.failure
					if len(requests) > 1 {
						failure = tc.retryError
					}
					if failure != "" {
						status := tc.status
						if status == 0 {
							status = http.StatusBadRequest
						}
						writeJSON(w, status, map[string]string{"message": failure})
						return
					}
					writeJSON(w, http.StatusCreated, map[string]string{"Id": "container-id"})
				}),
			)
			defer srv.Close()
			client, err := dockerC.NewClientWithOpts(
				dockerC.WithHost(srv.URL),
				dockerC.WithVersion("1.43"),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			rt := &DockerRuntime{
				Client: client, mgmt: m, config: clabruntime.RuntimeConfig{Timeout: time.Second},
			}
			id, err := rt.CreateContainer(context.Background(), node)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("CreateContainer() error = %v, want %q", err, tc.wantError)
				}
			} else if err != nil || id != "container-id" {
				t.Errorf("CreateContainer() = %q, %v, want successful creation", id, err)
			}
			if len(requests) != tc.wantCalls {
				t.Fatalf("ContainerCreate calls = %d, want %d", len(requests), tc.wantCalls)
			}
			if requests[0].IPv4Address != first4 || requests[0].IPv6Address != first6 {
				t.Errorf("first request lost allocated addresses: %+v", requests[0])
			}
			if tc.wantCalls == 2 &&
				(requests[1].IPv4Address != tc.ipv4 || requests[1].IPv6Address != tc.ipv6) {
				t.Errorf(
					"retry IPAM = %+v, want only explicit addresses %q, %q",
					requests[1],
					tc.ipv4,
					tc.ipv6,
				)
			}
		})
	}
}

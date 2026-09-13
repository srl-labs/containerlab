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

	dockerC "github.com/docker/docker/client"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

func TestNetworkAddressesFiltersPoolsBeforeInspect(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "inspect-error"}[failure], func(t *testing.T) {
			calls := map[string]int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/v1.43")
				calls[path]++
				w.Header().Set("Content-Type", "application/json")
				switch path {
				case "/networks":
					json.NewEncoder(w).Encode([]any{
						map[string]any{"Id": "unrelated", "Name": "unrelated", "IPAM": map[string]any{"Config": []any{map[string]string{"Subnet": "198.51.100.0/24"}}}},
						map[string]any{"Id": "match", "Name": "match", "IPAM": map[string]any{"Config": []any{map[string]string{"Subnet": "192.0.2.0/25"}, map[string]string{"Subnet": "2001:db8::/64"}}}},
						map[string]any{"Id": "host", "Name": "host"},
					})
				case "/networks/match":
					if failure {
						http.Error(w, `{"message":"inspection failed"}`, 500)
						return
					}
					json.NewEncoder(w).Encode(map[string]any{
						"Id": "match", "Name": "match",
						"IPAM":       map[string]any{"Config": []any{map[string]any{"Subnet": "192.0.2.0/25", "Gateway": "192.0.2.1", "AuxiliaryAddresses": map[string]string{"reserved": "192.0.2.3"}}, map[string]string{"Subnet": "2001:db8::/64", "Gateway": "2001:db8::1"}}},
						"Containers": map[string]any{"owner-id": map[string]string{"Name": "r1", "IPv4Address": "192.0.2.2/25", "IPv6Address": "2001:db8::2/64"}},
					})
				default:
					t.Errorf("unexpected inspection %s", path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			client, err := dockerC.NewClientWithOpts(dockerC.WithHost(srv.URL), dockerC.WithVersion("1.43"))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			d := &DockerRuntime{Client: client, config: clabruntime.RuntimeConfig{Timeout: time.Second}}
			got, err := d.NetworkAddresses(context.Background(), []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/64")})
			if failure {
				if err == nil {
					t.Fatal("inspection error ignored")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if calls["/networks"] != 1 || calls["/networks/match"] != 1 || len(calls) != 2 {
				t.Fatalf("wrong request count: %v", calls)
			}
			want := map[string]string{"192.0.2.1": "", "192.0.2.3": "", "2001:db8::1": "", "192.0.2.2": "owner-id", "2001:db8::2": "owner-id"}
			if len(got) != len(want) {
				t.Fatalf("snapshot: %+v", got)
			}
			for _, entry := range got {
				owner, ok := want[entry.Address.String()]
				if !ok || owner != entry.ContainerID || entry.NetworkName != "match" {
					t.Fatalf("wrong reservation: %+v", entry)
				}
			}
		})
	}
}

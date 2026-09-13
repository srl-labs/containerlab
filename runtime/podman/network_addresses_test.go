//go:build linux && podman

package podman

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	clabruntime "github.com/srl-labs/containerlab/runtime"
	"go.podman.io/podman/v6/pkg/bindings"
)

func TestNetworkAddressesFiltersPoolsBeforeInspect(t *testing.T) {
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/_ping") {
			w.Header().Set("Libpod-API-Version", "6.1.0")
			w.WriteHeader(200)
			return
		}
		if i := strings.Index(path, "/libpod/"); i >= 0 {
			path = path[i+len("/libpod"):]
		}
		calls[path]++
		w.Header().Set("Content-Type", "application/json")
		switch path {
		case "/networks/json":
			json.NewEncoder(w).Encode([]any{
				map[string]any{"id": "unrelated", "name": "unrelated", "subnets": []any{map[string]string{"subnet": "198.51.100.0/24"}}},
				map[string]any{"id": "match", "name": "match", "subnets": []any{map[string]string{"subnet": "192.0.2.0/25"}, map[string]string{"subnet": "2001:db8::/64"}}},
			})
		case "/networks/match/json":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "match", "name": "match", "subnets": []any{map[string]string{"subnet": "192.0.2.0/25", "gateway": "192.0.2.1"}},
				"containers": map[string]any{"owner-id": map[string]any{"name": "r1", "interfaces": map[string]any{"eth0": map[string]any{"subnets": []any{map[string]string{"ipnet": "192.0.2.2/25"}, map[string]string{"ipnet": "2001:db8::2/64"}}}}}},
			})
		default:
			t.Errorf("unexpected request %s", path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx, err := bindings.NewConnection(context.Background(), strings.Replace(srv.URL, "http://", "tcp://", 1))
	if err != nil {
		t.Fatal(err)
	}
	r := &PodmanRuntime{config: &clabruntime.RuntimeConfig{Timeout: time.Second}}
	got, err := r.NetworkAddresses(ctx, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8::/64")})
	if err != nil {
		t.Fatal(err)
	}
	if calls["/networks/json"] != 1 || calls["/networks/match/json"] != 1 || len(calls) != 2 {
		t.Fatalf("wrong request counts: %v", calls)
	}
	want := map[string]string{"192.0.2.1": "", "192.0.2.2": "owner-id", "2001:db8::2": "owner-id"}
	if len(got) != len(want) {
		t.Fatalf("snapshot: %+v", got)
	}
	for _, entry := range got {
		owner, ok := want[entry.Address.String()]
		if !ok || owner != entry.ContainerID || entry.NetworkName != "match" {
			t.Fatalf("wrong reservation: %+v", entry)
		}
	}
}

//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	clabruntime "github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	netTypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/bindings"
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

func TestNetworkAddressesFiltersPoolsBeforeInspect(t *testing.T) {
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/_ping") {
			w.Header().Set("Libpod-API-Version", "6.1.0")
			w.WriteHeader(http.StatusOK)
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
				map[string]any{
					"id":      "unrelated",
					"name":    "unrelated",
					"subnets": []any{map[string]string{"subnet": "198.51.100.0/24"}},
				},
				map[string]any{
					"id":   "match",
					"name": "match",
					"subnets": []any{
						map[string]string{"subnet": "192.0.2.0/25"},
						map[string]string{"subnet": "2001:db8::/64"},
					},
				},
			})
		case "/networks/match/json":
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "match",
				"name": "match",
				"subnets": []any{
					map[string]string{"subnet": "192.0.2.0/25", "gateway": "192.0.2.1"},
				},
				"containers": map[string]any{
					"owner-id": map[string]any{
						"name": "r1",
						"interfaces": map[string]any{
							"eth0": map[string]any{
								"subnets": []any{
									map[string]string{"ipnet": "192.0.2.2/25"},
									map[string]string{"ipnet": "2001:db8::2/64"},
								},
							},
						},
					},
				},
			})
		default:
			t.Errorf("unexpected request %s", path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx, err := bindings.NewConnection(
		context.Background(),
		strings.Replace(srv.URL, "http://", "tcp://", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	r := &PodmanRuntime{config: &clabruntime.RuntimeConfig{Timeout: time.Second}}
	got, err := r.NetworkAddresses(
		ctx,
		[]netip.Prefix{
			netip.MustParsePrefix("192.0.2.0/24"),
			netip.MustParsePrefix("2001:db8::/64"),
		},
	)
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

func TestNetOptsLeaseRanges(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{
		Network:    "clab",
		IPv4Subnet: "192.0.2.0/24",
		IPv4Range:  "192.0.2.128/26",
		IPv6Subnet: "2001:db8::/64",
		IPv6Range:  "2001:db8::100/126",
	}}
	network, err := r.netOpts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range [][2]string{
		{"192.0.2.129", "192.0.2.191"},
		{"2001:db8::101", "2001:db8::103"},
	} {
		lease := network.Subnets[i].LeaseRange
		if lease == nil || lease.StartIP.String() != want[0] || lease.EndIP.String() != want[1] {
			t.Fatalf("lease range %d = %+v; want %v", i, lease, want)
		}
	}

	r.mgmt.IPv4Range = "198.51.100.0/24"
	if _, err := r.netOpts(context.Background()); err == nil {
		t.Fatal("accepted an IP range outside its subnet")
	}
}

func TestSetMgmtIPAMFromPodmanSubnets(t *testing.T) {
	v4, err := netTypes.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	v6, err := netTypes.ParseCIDR("2001:db8::/64")
	if err != nil {
		t.Fatal(err)
	}
	m := &types.MgmtNet{IPv4Range: "198.51.100.0/24", IPv6Range: "2001:db8:1::/64"}
	err = setMgmtIPAMFromPodmanSubnets(m, []netTypes.Subnet{
		{
			Subnet:  v4,
			Gateway: net.ParseIP("192.0.2.1"),
			LeaseRange: &netTypes.LeaseRange{
				StartIP: net.ParseIP("192.0.2.129"),
				EndIP:   net.ParseIP("192.0.2.191"),
			},
		},
		{Subnet: v6, Gateway: net.ParseIP("2001:db8::1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.IPv4Subnet != "192.0.2.0/24" || m.IPv4Gw != "192.0.2.1" ||
		m.IPv4Range != "192.0.2.128/26" {
		t.Fatalf("incorrect IPv4 IPAM: %+v", m)
	}
	if m.IPv6Subnet != "2001:db8::/64" || m.IPv6Gw != "2001:db8::1" ||
		m.IPv6Range != "" {
		t.Fatalf("incorrect IPv6 IPAM: %+v", m)
	}

	err = setMgmtIPAMFromPodmanSubnets(m, []netTypes.Subnet{{
		Subnet: v4,
		LeaseRange: &netTypes.LeaseRange{
			StartIP: net.ParseIP("192.0.2.20"),
			EndIP:   net.ParseIP("192.0.2.50"),
		},
	}})
	if err == nil {
		t.Fatal("accepted a lease range that cannot be represented as a CIDR")
	}
}

func TestCreateNetValidatesManagementConfig(t *testing.T) {
	r := &PodmanRuntime{mgmt: &types.MgmtNet{
		IPAM: types.MgmtIPAM{Provider: types.IPAMProvider("invalid")},
	}}
	if err := r.CreateNet(context.Background()); err == nil {
		t.Fatal("accepted an invalid IPAM provider")
	}
}

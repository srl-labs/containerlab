package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestStateStoresPreferredAllocationsSeparatelyFromTopology(t *testing.T) {
	paths := &clabtypes.TopoPaths{}
	if err := paths.SetLabDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	cfg := &clabtypes.NodeConfig{
		ShortName:       "node",
		MgmtIPv4Address: "192.0.2.5",
		MgmtIPv6Address: "2001:db8::5",
	}
	ctrl := gomock.NewController(t)
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(cfg).AnyTimes()
	topology := clabtypes.NewTopology()
	topology.Nodes["node"] = &clabtypes.NodeDefinition{Kind: "linux"}
	c := &CLab{
		TopoPaths: paths,
		Config: &Config{
			Topology: topology,
			Mgmt:     &clabtypes.MgmtNet{IPv4Subnet: "192.0.2.0/24", IPv6Subnet: "2001:db8::/64"},
		},
		Nodes: map[string]clabnodes.Node{"node": node},
	}
	if err := c.WriteState(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths.StateFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "ipam:\n  node:\n    ipv4: 192.0.2.5\n") {
		t.Fatalf("unexpected state format:\n%s", data)
	}
	state, err := c.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	preferred := state.IPAM["node"]
	if preferred.IPv4 != cfg.MgmtIPv4Address || preferred.IPv6 != cfg.MgmtIPv6Address {
		t.Fatalf("missing preferred allocation: %+v", preferred)
	}
	if state.Topology.Nodes["node"].MgmtIPv4 != "" || topology.Nodes["node"].MgmtIPv4 != "" {
		t.Fatal("allocation preferences changed desired topology")
	}
	// A no-op apply can have unpopulated runtime fields; preserve its prior preferences.
	cfg.MgmtIPv4Address, cfg.MgmtIPv6Address = "", ""
	if err := c.WriteState(); err != nil {
		t.Fatal(err)
	}
	state, err = c.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.IPAM["node"].IPv4 != "192.0.2.5" {
		t.Fatal("no-op write lost allocation")
	}
	// Topology changes do not discard the last allocated address.
	topology.Nodes["node"].MgmtIPv4 = "192.0.2.9"
	if err := c.WriteState(); err != nil {
		t.Fatal(err)
	}
	state, err = c.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.IPAM["node"].IPv4 != "192.0.2.5" ||
		state.Topology.Nodes["node"].MgmtIPv4 != "192.0.2.9" {
		t.Fatal("topology change discarded preferred allocation")
	}
	entries, err := os.ReadDir(filepath.Dir(paths.StateFile()))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatal("temporary state file leaked")
		}
	}
	// Deleted nodes must not keep preferences indefinitely.
	delete(c.Nodes, "node")
	if err := c.WriteState(); err != nil {
		t.Fatal(err)
	}
	state, err = c.LoadState()
	if err != nil || len(state.IPAM) != 0 {
		t.Fatalf("deleted node retained: %+v, %v", state, err)
	}
}

func TestLegacyStateWithoutAllocationPreferences(t *testing.T) {
	paths := &clabtypes.TopoPaths{}
	if err := paths.SetLabDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		paths.StateFile(),
		[]byte("topology:\n  nodes:\n    node:\n      kind: linux\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}
	c := &CLab{TopoPaths: paths}
	state, err := c.LoadState()
	if err != nil || state.Topology.Nodes["node"].Kind != "linux" || len(state.IPAM) != 0 {
		t.Fatalf("legacy state: %+v, %v", state, err)
	}
}

package core

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

// newMigrationMockNode returns a mocked node for the c.Nodes inventory;
// name-encoded "<bridge>|<parent>" names get the BridgeNS endpoint type.
func newMigrationMockNode(t *testing.T, name string) clabnodes.Node {
	t.Helper()

	ctrl := gomock.NewController(t)
	node := clabmocksmocknodes.NewMockNode(ctrl)
	node.EXPECT().Config().Return(&clabtypes.NodeConfig{
		ShortName: name,
		Kind:      "bridge",
	}).AnyTimes()
	node.EXPECT().GetShortName().Return(name).AnyTimes()
	if name != "" {
		node.EXPECT().GetLinkEndpointType().
			Return(clablinks.LinkEndpointType("bridge-ns")).AnyTimes()
	}

	return node
}

// migrationGuardFixture describes the desired topology for guard tests. Each
// desiredVeths entry is (nodeA, ifaceA, nodeB, ifaceB); fake link nodes back
// the client endpoints.
type migrationGuardFixture struct {
	desiredVeths [][4]string
	deleted      string
	recreated    string
	knownNodes   []string // parents registered in c.Nodes
}

func (f migrationGuardFixture) build(
	t *testing.T,
	desiredSet map[applyEndpointKey]struct{},
) (*CLab, *applyPlan) {
	t.Helper()

	c := &CLab{
		Nodes: map[string]clabnodes.Node{},
		Links: map[int]clablinks.Link{},
	}
	for _, parent := range f.knownNodes {
		c.Nodes[parent] = newMigrationMockNode(t, parent)
	}
	c.Nodes["br01|bp2"] = newMigrationMockNode(t, "br01|bp2")

	linkNodes := map[string]*applyFakeLinkNode{}
	for i, v := range f.desiredVeths {
		nodeA, ok := linkNodes[v[0]]
		if !ok {
			nodeA = &applyFakeLinkNode{name: v[0]}
			linkNodes[v[0]] = nodeA
		}
		nodeB, ok := linkNodes[v[2]]
		if !ok {
			nodeB = &applyFakeLinkNode{name: v[2]}
			linkNodes[v[2]] = nodeB
		}
		link := clablinks.NewLinkVEth()
		link.Endpoints = []clablinks.Endpoint{
			clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(nodeA, v[1], link)),
			clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(nodeB, v[3], link)),
		}
		c.Links[i] = link
	}

	// Desired links contribute every endpoint, client side included.
	for _, link := range c.Links {
		for _, ep := range clablinks.RuntimeEndpoints(link) {
			desiredSet[applyEndpointKey{
				node:  ep.GetNode().GetShortName(),
				iface: ep.GetIfaceName(),
			}] = struct{}{}
		}
	}

	plan := newApplyPlan(nil, nil)
	plan.desiredEndpointSet = desiredSet
	if f.deleted != "" {
		plan.deletedNodeSet = map[string]struct{}{f.deleted: {}}
	}
	if f.recreated != "" {
		plan.recreatedNodeSet = map[string]struct{}{f.recreated: {}}
	}

	return c, plan
}

func TestAddBridgeNSMigrationCandidateGuard(t *testing.T) {
	t.Parallel()

	var (
		brStale       = applyEndpointKey{node: "br01|bp1", iface: "c1eth1"}
		brDesired     = applyEndpointKey{node: "br01|bp2", iface: "c1eth1"}
		clientEth1    = applyEndpointKey{node: "c1", iface: "eth1"}
		defaultParent = []string{"bp1", "bp2"}
	)

	tests := []struct {
		name        string
		fixture     migrationGuardFixture
		staleClient applyEndpointKey
		wantPlan    bool
	}{
		{
			name: "unchanged client peer plans migration",
			fixture: migrationGuardFixture{
				desiredVeths: [][4]string{{"c1", "eth1", "br01|bp2", "c1eth1"}},
				knownNodes:   defaultParent,
			},
			staleClient: clientEth1,
			wantPlan:    true,
		},
		{
			name: "rewired client endpoint skips migration",
			fixture: migrationGuardFixture{
				// c1:eth1 stays desired behind a different port; the
				// migrated port is claimed by another client.
				desiredVeths: [][4]string{
					{"c1", "eth1", "c10", "other"},
					{"c2", "eth3", "br01|bp2", "c1eth1"},
				},
				knownNodes: defaultParent,
			},
			staleClient: clientEth1,
			wantPlan:    false,
		},
		{
			name: "deleted client node skips migration",
			fixture: migrationGuardFixture{
				desiredVeths: [][4]string{{"c1", "eth1", "br01|bp2", "c1eth1"}},
				deleted:      "c1",
				knownNodes:   defaultParent,
			},
			staleClient: clientEth1,
			wantPlan:    false,
		},
		{
			name: "recreated client node skips migration",
			fixture: migrationGuardFixture{
				desiredVeths: [][4]string{{"c1", "eth1", "br01|bp2", "c1eth1"}},
				recreated:    "c1",
				knownNodes:   defaultParent,
			},
			staleClient: clientEth1,
			wantPlan:    false,
		},
		{
			name: "uncatalogued client endpoint skips migration",
			fixture: migrationGuardFixture{
				// The migrated port is claimed by c2; c1:eth1 is desired
				// nowhere, so the planning claim cannot be validated.
				desiredVeths: [][4]string{{"c2", "eth3", "br01|bp2", "c1eth1"}},
				knownNodes:   defaultParent,
			},
			staleClient: clientEth1,
			wantPlan:    false,
		},
		{
			name: "unresolvable old bridge parent skips migration",
			fixture: migrationGuardFixture{
				desiredVeths: [][4]string{{"c1", "eth1", "br01|bp2", "c1eth1"}},
				knownNodes:   []string{"bp2"},
			},
			staleClient: clientEth1,
			wantPlan:    false,
		},
		{
			name: "unknown client-side peer skips migration",
			fixture: migrationGuardFixture{
				desiredVeths: [][4]string{{"c1", "eth1", "br01|bp2", "c1eth1"}},
				knownNodes:   defaultParent,
			},
			staleClient: applyEndpointKey{},
			wantPlan:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, plan := tt.fixture.build(
				t, map[applyEndpointKey]struct{}{},
			)

			c.addBridgeNSMigrationCandidate(
				plan, brStale, nil, tt.staleClient,
			)

			if got := len(plan.bridgeNSMigrations); got != 0 && !tt.wantPlan {
				t.Fatalf("planned %d migrations for %q of %q; want none",
					got, tt.staleClient.String(), brStale.String())
			}
			if tt.wantPlan {
				if got := len(plan.bridgeNSMigrations); got != 1 {
					t.Fatalf("planned %d migrations; want 1", got)
				}
				if plan.bridgeNSMigrations[0].newKey != brDesired {
					t.Fatalf(
						"migration target = %q; want %q",
						plan.bridgeNSMigrations[0].newKey.String(),
						brDesired.String(),
					)
				}
			}
		})
	}
}

func TestCollectDesertedBridgeDevice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		bridgeName  string
		deleted     string
		recreated   string
		bridgeNode  string // recreated bridge node name, kept for reuse
		wantDevices []desertedBridgeDevice
	}{
		{
			name:        "collects deserted device for surviving parent",
			bridgeName:  "br01|bp1",
			wantDevices: []desertedBridgeDevice{{parentName: "bp1", deviceName: "br01"}},
		},
		{
			name:        "deduplicates repeated records",
			bridgeName:  "br01|bp1",
			wantDevices: []desertedBridgeDevice{{parentName: "bp1", deviceName: "br01"}},
		},
		{
			name:       "skips deletion when parent is deleted",
			bridgeName: "br01|bp1",
			deleted:    "bp1",
		},
		{
			name:       "skips deletion when parent is recreated",
			bridgeName: "br01|bp1",
			recreated:  "bp1",
		},
		{
			name:       "skips deletion when the bridge itself is recreated",
			bridgeName: "br01|bp1",
			bridgeNode: "br01|bp1",
		},
		{
			name:       "skips nameless parents",
			bridgeName: "br01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan := newApplyPlan(nil, nil)
			if tt.deleted != "" {
				plan.deletedNodeSet = map[string]struct{}{tt.deleted: {}}
			}
			if tt.recreated != "" {
				plan.recreatedNodeSet[tt.recreated] = struct{}{}
			}
			if tt.bridgeNode != "" {
				plan.recreatedNodeSet[tt.bridgeNode] = struct{}{}
			}

			c := &CLab{Nodes: map[string]clabnodes.Node{}}
			c.collectDesertedBridgeDevice(plan, tt.bridgeName)
			// Second call exercises dedupe for the repeat case.
			c.collectDesertedBridgeDevice(plan, tt.bridgeName)

			if tt.wantDevices == nil {
				if got := len(plan.desertedBridgeDevices); got != 0 {
					t.Fatalf("recorded %d deserted devices; want none", got)
				}
				return
			}

			if len(plan.desertedBridgeDevices) != len(tt.wantDevices) {
				t.Fatalf(
					"recorded %d deserted devices; want %d",
					len(plan.desertedBridgeDevices), len(tt.wantDevices),
				)
			}
			for i, want := range tt.wantDevices {
				if got := plan.desertedBridgeDevices[i]; got != want {
					t.Fatalf("record %d = %+v; want %+v", i, got, want)
				}
			}
		})
	}
}

func TestPlanDesertedBridgeDeviceRemovals(t *testing.T) {
	t.Parallel()

	c := &CLab{Nodes: map[string]clabnodes.Node{}}

	plan := newApplyPlan(nil, nil)
	plan.deletedNodeSet = map[string]struct{}{
		"br01|bp1": {}, // bridge removed, bp1 survives
		"br02|bp3": {}, // removed, but parent bp3 is deleted too
		"c9":       {}, // plain container node: not a BridgeNS name
	}
	plan.deletedNodeSet["bp3"] = struct{}{}
	plan.recreatedNodeSet["br04|bp1"] = struct{}{}
	plan.deletedNodeSet["br04|bp1"] = struct{}{} // recreate implies deletion
	plan.state = &LabState{
		Topology: &clabtypes.Topology{
			Nodes: map[string]*clabtypes.NodeDefinition{
				"br05|bp1": {}, // state-only removal, bp1 survives
				"c1":       {},
			},
		},
	}

	c.planDesertedBridgeDeviceRemovals(plan)

	want := []desertedBridgeDevice{
		{parentName: "bp1", deviceName: "br01"},
		{parentName: "bp1", deviceName: "br05"},
	}
	if len(plan.desertedBridgeDevices) != len(want) {
		t.Fatalf("recorded %d deserted devices; want %d", len(plan.desertedBridgeDevices), len(want))
	}
	for i, d := range want {
		if got := plan.desertedBridgeDevices[i]; got != d {
			t.Fatalf("record %d = %+v; want %+v", i, got, d)
		}
	}
}

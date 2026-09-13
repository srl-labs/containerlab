package links

import (
	"testing"
)

func TestIfacePortIndex(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name string
		port int
		ok   bool
	}{
		"srl":        {name: "e1-3", port: 3, ok: true},
		"eos":        {name: "et3", port: 3, ok: true},
		"frr":        {name: "eth3", port: 3, ok: true},
		"breakout":   {name: "e1-2-3", port: 3, ok: true},
		"two-digits": {name: "eth12", port: 12, ok: true},
		"no-digits":  {name: "lo", ok: false},
		"eth":        {name: "eth", ok: false},
		"empty":      {name: "", ok: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			port, ok := ifacePortIndex(tc.name)
			if ok != tc.ok || port != tc.port {
				t.Fatalf("ifacePortIndex(%q) = (%d, %v), want (%d, %v)", tc.name, port, ok, tc.port, tc.ok)
			}
		})
	}
}

func TestMapParkedPeerIndexesRenamesBothParkedEndsByPort(t *testing.T) {
	t.Parallel()

	// SRL → EOS group swap: both ends still named e1-3 in parking netns.
	got := mapParkedPeerIndexes(
		[]pendingPeerRename{{desiredName: "et3", peerName: "et3"}},
		[]indexedIface{{name: "e1-3", index: 42}},
	)
	if got[42] != "et3" {
		t.Fatalf("mapParkedPeerIndexes() = %v, want peer ifindex 42 → et3", got)
	}
}

func TestMapParkedPeerIndexesPairsMultipleLinksByTrailingPort(t *testing.T) {
	t.Parallel()

	got := mapParkedPeerIndexes(
		[]pendingPeerRename{
			{desiredName: "eth1", peerName: "eth1"},
			{desiredName: "eth3", peerName: "eth3"},
		},
		[]indexedIface{
			{name: "e1-3", index: 30},
			{name: "e1-1", index: 10},
		},
	)
	if got[10] != "eth1" || got[30] != "eth3" {
		t.Fatalf("mapParkedPeerIndexes() = %v, want 10→eth1 30→eth3", got)
	}
}

func TestMapParkedPeerIndexesPairsUniqueLeftoverWithoutPort(t *testing.T) {
	t.Parallel()

	got := mapParkedPeerIndexes(
		[]pendingPeerRename{{desiredName: "eth3", peerName: "eth3"}},
		[]indexedIface{{name: "veth-parked", index: 7}},
	)
	if got[7] != "eth3" {
		t.Fatalf("mapParkedPeerIndexes() = %v, want unique leftover 7 → eth3", got)
	}
}

func TestMapParkedPeerIndexesSkipsAmbiguousPorts(t *testing.T) {
	t.Parallel()

	got := mapParkedPeerIndexes(
		[]pendingPeerRename{
			{desiredName: "eth1", peerName: "eth1"},
			{desiredName: "eth2", peerName: "eth2"},
		},
		[]indexedIface{
			{name: "e1-1", index: 11},
			{name: "foo1", index: 12},
		},
	)
	if len(got) != 0 {
		t.Fatalf("mapParkedPeerIndexes() = %v, want no mapping for ambiguous port 1", got)
	}
}

package links

import (
	"context"
	"testing"
)

// mgmt-net endpoints of a management network that is not created yet have no bridge,
// so verification must not require the bridge to exist.
func TestMgmtNetEndpointUnresolvedBridge(t *testing.T) {
	const network = "mgmtnet-verify-unresolved"
	t.Cleanup(func() {
		_mgmtBrLinkNodesMu.Lock()
		delete(_mgmtBrLinkNodes, network)
		_mgmtBrLinkNodesMu.Unlock()
	})

	node := getMgmtBrLinkNode(network)
	if node == nil {
		t.Fatal("failed to get the mgmt-net link node")
	}

	for _, tc := range []struct {
		name       string
		bridge     string
		mgmt       bool
		unresolved bool
	}{
		{name: "bridge not set", bridge: mgmtNetOwnerName, mgmt: true, unresolved: true},
		{name: "network not created", bridge: "", mgmt: true, unresolved: true},
		{name: "bridge resolved", bridge: "br-lab", mgmt: true, unresolved: false},
		{name: "regular bridge", bridge: "", mgmt: false, unresolved: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := SetMgmtNetworkUnderlyingBridge(network, tc.bridge); err != nil {
				t.Fatal(err)
			}
			ep := NewEndpointBridge(NewEndpointGeneric(node, "r1-eth1", nil), tc.mgmt)

			if got := IsUnresolvedMgmtBridgeEndpoint(ep); got != tc.unresolved {
				t.Fatalf("IsUnresolvedMgmtBridgeEndpoint() = %v, want %v", got, tc.unresolved)
			}
			if !tc.unresolved {
				return
			}

			params := NewVerifyLinkParams()
			params.RunBridgeExistsCheck = true
			params.AllowExistingEndpoint = true
			if err := ep.Verify(context.Background(), params); err != nil {
				t.Fatalf("Verify() = %v, want nil", err)
			}
		})
	}
}

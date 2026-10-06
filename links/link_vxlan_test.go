package links

import "testing"

func TestResolveVxlanStitchedSanitizesHostIfaceNames(t *testing.T) {
	tests := []struct {
		name      string
		iface     string
		overwrite string
		wantVx    string
		wantVe    string
	}{
		{
			name:   "plain interface name",
			iface:  "eth1",
			wantVx: "vx-d1_eth1",
			wantVe: "ve-d1_eth1",
		},
		{
			name:   "interface name with slashes",
			iface:  "1/1/c3/1",
			wantVx: "vx-d1_1-1-c3-1",
			wantVe: "ve-d1_1-1-c3-1",
		},
		{
			name:      "overwritten interface name",
			iface:     "1/1/c3/1",
			overwrite: "eth9",
			wantVx:    "vx-eth9",
			wantVe:    "eth9",
		},
		{
			name:      "overwritten interface name with slashes",
			iface:     "1/1/c3/1",
			overwrite: "a/b",
			wantVx:    "vx-a-b",
			wantVe:    "a-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LinkVxlanRaw{
				LinkType: LinkTypeVxlanStitch,
				Remote:   "192.0.2.1",
				VNI:      7,
				// set explicitly to skip the host route lookup for the remote address
				ParentInterface: "eth0",
				Endpoint:        EndpointRaw{Node: "d1", Iface: tt.iface},
			}

			params := &ResolveParams{
				Nodes: map[string]Node{
					"d1":   newFakeNode("d1"),
					"host": GetHostLinkNode(),
				},
				VxlanIfaceNameOverwrite: tt.overwrite,
			}

			vxs, err := r.ResolveStitched(params)
			if err != nil {
				t.Fatalf("ResolveStitched() error = %v", err)
			}

			if got := vxs.vxlanLink.localEndpoint.GetIfaceName(); got != tt.wantVx {
				t.Errorf("vxlan host iface = %q, want %q", got, tt.wantVx)
			}
			if got := vxs.vethStitchEp.GetIfaceName(); got != tt.wantVe {
				t.Errorf("veth host iface = %q, want %q", got, tt.wantVe)
			}
		})
	}
}

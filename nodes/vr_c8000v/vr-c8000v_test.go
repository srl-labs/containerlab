package vr_c8000v

import (
	"testing"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestC8000vInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		endpoints []*clablinks.EndpointVeth
		node      *vrC8000v
		resultEps []string
	}{
		"alias-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "Gi2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "GigabitEthernet 6",
					},
				},
			},
			node: &vrC8000v{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "c8000v",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth1", "eth3", "eth5",
			},
		},
		"original-parse": {
			endpoints: []*clablinks.EndpointVeth{
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth2",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth4",
					},
				},
				{
					EndpointGeneric: clablinks.EndpointGeneric{
						IfaceName: "eth6",
					},
				},
			},
			node: &vrC8000v{
				VRNode: clabnodes.VRNode{
					DefaultNode: clabnodes.DefaultNode{
						Cfg: &clabtypes.NodeConfig{
							ShortName: "c8000v",
						},
						InterfaceRegexp: InterfaceRegexp,
						InterfaceOffset: InterfaceOffset,
					},
				},
			},
			resultEps: []string{
				"eth2", "eth4", "eth6",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(tt *testing.T) {
			foundError := false
			tc.node.OverwriteNode = tc.node
			tc.node.InterfaceMappedPrefix = "eth"
			tc.node.FirstDataIfIndex = 1
			for _, ep := range tc.endpoints {
				gotEndpointErr := tc.node.AddEndpoint(ep)
				if gotEndpointErr != nil {
					foundError = true
					t.Errorf("got error for endpoint %+v", gotEndpointErr)
				}
			}

			if !foundError {
				gotCheckErr := tc.node.CheckInterfaceName()
				if gotCheckErr != nil {
					foundError = true
					t.Errorf("got error for check %+v", gotCheckErr)
				}

				if !foundError {
					for idx, ep := range tc.node.Endpoints {
						if ep.GetIfaceName() != tc.resultEps[idx] {
							t.Errorf("got wrong mapped endpoint %q (%q), want %q",
								ep.GetIfaceName(), ep.GetIfaceAlias(), tc.resultEps[idx])
						}
					}
				}
			}
		})
	}
}

func initC8000v(t *testing.T, cfg *clabtypes.NodeConfig) (*vrC8000v, error) {
	t.Helper()
	cfg.LabDir = t.TempDir()
	cfg.ShortName = "c8000v"
	n := new(vrC8000v)
	err := n.Init(cfg, clabnodes.WithMgmtNet(&clabtypes.MgmtNet{}))
	return n, err
}

func TestC8000vInterfaceMappingByNetworkMode(t *testing.T) {
	tests := map[string]struct {
		networkMode string
		ifaces      []string
		want        []string
		wantErr     bool
	}{
		"default-maps-gi2-to-eth1": {
			ifaces: []string{"Gi2", "GigabitEthernet3"},
			want:   []string{"eth1", "eth2"},
		},
		"default-rejects-gi1": {
			ifaces:  []string{"Gi1"},
			wantErr: true,
		},
		"network-mode-none-maps-gi1-to-eth1": {
			networkMode: "none",
			ifaces:      []string{"Gi1", "GigabitEthernet2", "eth3"},
			want:        []string{"eth1", "eth2", "eth3"},
		},
		"network-mode-container-keeps-mgmt": {
			networkMode: "container:other",
			ifaces:      []string{"Gi2"},
			want:        []string{"eth1"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n, err := initC8000v(t, &clabtypes.NodeConfig{NetworkMode: tc.networkMode})
			if err != nil {
				t.Fatalf("Init() failed: %v", err)
			}
			n.InterfaceMappedPrefix = "eth"
			n.FirstDataIfIndex = 1

			for _, ifName := range tc.ifaces {
				ep := &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: ifName},
				}
				if err = n.AddEndpoint(ep); err != nil {
					break
				}
			}
			if err == nil {
				err = n.CheckInterfaceName()
			}

			if tc.wantErr {
				if err == nil {
					t.Fatal("interface mapping succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("interface mapping failed: %v", err)
			}
			for i, ep := range n.Endpoints {
				if ep.GetIfaceName() != tc.want[i] {
					t.Errorf("%q mapped to %q, want %q",
						tc.ifaces[i], ep.GetIfaceName(), tc.want[i])
				}
			}
		})
	}
}

func TestC8000vModeValidation(t *testing.T) {
	tests := map[string]struct {
		cfg      clabtypes.NodeConfig
		wantMode string
		wantErr  bool
	}{
		"default-mode": {
			wantMode: modeAutonomous,
		},
		"controller-mode": {
			cfg:      clabtypes.NodeConfig{NodeType: modeController},
			wantMode: modeController,
		},
		"invalid-mode": {
			cfg:     clabtypes.NodeConfig{NodeType: "bogus"},
			wantErr: true,
		},
		"autonomous-with-startup-config": {
			cfg:      clabtypes.NodeConfig{StartupConfig: "cfg.txt"},
			wantMode: modeAutonomous,
		},
		"ztp-without-startup-config": {
			cfg:      clabtypes.NodeConfig{NodeType: modeZTP},
			wantMode: modeZTP,
		},
		"ztp-rejects-startup-config": {
			cfg:     clabtypes.NodeConfig{NodeType: modeZTP, StartupConfig: "cfg.txt"},
			wantErr: true,
		},
		"ztp-allows-suppressed-startup-config": {
			cfg: clabtypes.NodeConfig{
				NodeType:              modeZTP,
				StartupConfig:         "cfg.txt",
				SuppressStartupConfig: true,
			},
			wantMode: modeZTP,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n, err := initC8000v(t, &tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Init() succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Init() failed: %v", err)
			}
			if n.mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", n.mode, tc.wantMode)
			}
			if got := n.Cfg.Env["MODE"]; got != tc.wantMode {
				t.Errorf("MODE env = %q, want %q", got, tc.wantMode)
			}
		})
	}
}

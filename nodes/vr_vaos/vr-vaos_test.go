package vr_vaos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func newTestNode(t *testing.T) *vrVAOS {
	t.Helper()

	n := &vrVAOS{
		VRNode: clabnodes.VRNode{
			DefaultNode: clabnodes.DefaultNode{
				Cfg: &clabtypes.NodeConfig{
					ShortName: "vaos",
					LabDir:    t.TempDir(),
				},
				InterfaceRegexp: InterfaceRegexp,
				InterfaceOffset: InterfaceOffset,
			},
			ConfigDirName:   "config",
			StartupCfgFName: "startup-config.cfg",
		},
	}
	n.OverwriteNode = n
	n.InterfaceMappedPrefix = "eth"
	n.FirstDataIfIndex = 1

	return n
}

func TestVAOSInterfaceParsing(t *testing.T) {
	tests := map[string]struct {
		ifaces  []string
		want    []string
		wantErr bool
	}{
		"alias-parse": {
			ifaces: []string{"1/1/1", "1/1/3", "1/1/10"},
			want:   []string{"eth1", "eth3", "eth10"},
		},
		"original-parse": {
			ifaces: []string{"eth2", "eth11", "eth12"},
			want:   []string{"eth2", "eth11", "eth12"},
		},
		"alias-out-of-range": {
			ifaces:  []string{"1/1/11"},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newTestNode(t)

			var err error

			for _, iface := range tc.ifaces {
				ep := &clablinks.EndpointVeth{
					EndpointGeneric: clablinks.EndpointGeneric{IfaceName: iface},
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
					t.Fatalf("expected an error for %v", tc.ifaces)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for idx, ep := range n.Endpoints {
				if ep.GetIfaceName() != tc.want[idx] {
					t.Errorf("got %q, want %q", ep.GetIfaceName(), tc.want[idx])
				}
			}
		})
	}
}

// fakeLauncher answers a save request the way launch.py does.
func fakeLauncher(t *testing.T, cfgDir, result string) {
	t.Helper()

	go func() {
		req := filepath.Join(cfgDir, saveRequestFile)
		for {
			if _, err := os.Stat(req); err == nil {
				break
			}

			time.Sleep(10 * time.Millisecond)
		}

		_ = os.Remove(req)
		// the done file exists, empty, before the result is written
		_ = os.WriteFile(filepath.Join(cfgDir, saveDoneFile), nil, 0o644)

		time.Sleep(50 * time.Millisecond)

		_ = os.WriteFile(filepath.Join(cfgDir, saveDoneFile), []byte(result+"\n"), 0o644)
	}()
}

func TestVAOSSaveConfig(t *testing.T) {
	tests := map[string]struct {
		result  string
		wantErr bool
	}{
		"ok":    {result: "ok"},
		"error": {result: "error: could not log in on the console", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n := newTestNode(t)

			cfgDir := filepath.Join(n.Cfg.LabDir, n.ConfigDirName)
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatal(err)
			}

			fakeLauncher(t, cfgDir, tc.result)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			res, err := n.SaveConfig(ctx)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			want := filepath.Join(cfgDir, "startup-config.cfg")
			if res == nil || res.ConfigPath != want {
				t.Errorf("ConfigPath = %v, want %q", res, want)
			}
		})
	}
}

func TestVAOSSaveConfigTimeout(t *testing.T) {
	doneFile := filepath.Join(t.TempDir(), saveDoneFile)

	_, err := waitSaveDone(context.Background(), doneFile, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}

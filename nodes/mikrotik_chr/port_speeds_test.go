package mikrotik_chr

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestPortSpeeds(t *testing.T) {
	tests := map[string]struct {
		speeds   []PortSpeed
		lastPort int    // ether64 on amd64, ether30 on arm64
		want     string // the CHR_PORT_SPEEDS value
		errText  string // empty when the key is accepted
	}{
		"none": {lastPort: 64, want: ""},
		"single-port": {
			speeds:   []PortSpeed{{Port: "26", Speed: 10000}},
			lastPort: 64,
			want:     "26:10000",
		},
		"range": {
			speeds:   []PortSpeed{{Port: "2..4", Speed: 1000}},
			lastPort: 64,
			want:     "2:1000 3:1000 4:1000",
		},
		"sorted-by-port": {
			speeds:   []PortSpeed{{Port: "18..19", Speed: 100000}, {Port: "2..3", Speed: 25000}},
			lastPort: 64,
			want:     "2:25000 3:25000 18:100000 19:100000",
		},
		"amd64-last-port": {
			speeds:   []PortSpeed{{Port: "64", Speed: 1000}},
			lastPort: 64,
			want:     "64:1000",
		},
		"arm64-last-port": {
			speeds:   []PortSpeed{{Port: "30", Speed: 1000}},
			lastPort: 30,
			want:     "30:1000",
		},
		"max-speed": {
			speeds:   []PortSpeed{{Port: "2", Speed: 2147483647}},
			lastPort: 64,
			want:     "2:2147483647",
		},
		"management-port": {
			speeds:   []PortSpeed{{Port: "1", Speed: 1000}},
			lastPort: 64,
			errText: `port-speeds: port "1" must be a port number or an ascending range ` +
				"within 2..64 (ether2 to ether64)",
		},
		"range-from-management": {
			speeds:   []PortSpeed{{Port: "1..4", Speed: 1000}},
			lastPort: 64,
			errText:  "within 2..64",
		},
		"amd64-over-limit": {
			speeds:   []PortSpeed{{Port: "65", Speed: 1000}},
			lastPort: 64,
			errText:  "within 2..64",
		},
		"arm64-over-limit": {
			speeds:   []PortSpeed{{Port: "2..31", Speed: 1000}},
			lastPort: 30,
			errText:  "within 2..30 (ether2 to ether30)",
		},
		"descending": {
			speeds:   []PortSpeed{{Port: "25..2", Speed: 1000}},
			lastPort: 64,
			errText:  "ascending range",
		},
		"leading-zero": {
			speeds:   []PortSpeed{{Port: "07", Speed: 1000}},
			lastPort: 64,
			errText:  `port "07"`,
		},
		"name-not-number": {
			speeds:   []PortSpeed{{Port: "ether2", Speed: 1000}},
			lastPort: 64,
			errText:  `port "ether2"`,
		},
		"empty-port": {
			speeds:   []PortSpeed{{Speed: 1000}},
			lastPort: 64,
			errText:  `port ""`,
		},
		"half-range": {
			speeds:   []PortSpeed{{Port: "2..", Speed: 1000}},
			lastPort: 64,
			errText:  `port "2.."`,
		},
		"overlap": {
			speeds:   []PortSpeed{{Port: "2..10", Speed: 1000}, {Port: "10", Speed: 10000}},
			lastPort: 64,
			errText:  "port-speeds: ether10 is set more than once",
		},
		"missing-speed": {
			speeds:   []PortSpeed{{Port: "2"}},
			lastPort: 64,
			errText:  `port-speeds: port "2": speed must be between 1 and 2147483647 Mb/s, got 0`,
		},
		"negative-speed": {
			speeds:   []PortSpeed{{Port: "2", Speed: -1}},
			lastPort: 64,
			errText:  "speed must be between 1 and 2147483647",
		},
		"over-int32": {
			speeds:   []PortSpeed{{Port: "2", Speed: 2147483648}},
			lastPort: 64,
			errText:  "got 2147483648",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := portSpeeds(&KindSpecificConfig{PortSpeeds: tc.speeds}, tc.lastPort)
			if tc.errText == "" {
				if err != nil {
					t.Fatalf("portSpeeds(%v) = %v, want no error", tc.speeds, err)
				}
				if got != tc.want {
					t.Errorf("portSpeeds(%v) = %q, want %q", tc.speeds, got, tc.want)
				}

				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.errText) {
				t.Errorf(
					"portSpeeds(%v) = %v, want an error containing %q",
					tc.speeds,
					err,
					tc.errText,
				)
			}
		})
	}
}

// TestDecodePortSpeeds decodes the key from YAML the way a topology sets it: a single port may be
// written as a number, a range as a string, and a speed must be a number.
func TestDecodePortSpeeds(t *testing.T) {
	tests := map[string]struct {
		yaml    string
		want    []PortSpeed
		errText string
	}{
		"number-and-range": {
			yaml: "- port: 26\n  speed: 10000\n- port: 2..25\n  speed: 1000\n",
			want: []PortSpeed{{Port: "26", Speed: 10000}, {Port: "2..25", Speed: 1000}},
		},
		"speed-with-unit": {
			yaml:    "- port: 2\n  speed: 1G\n",
			errText: `invalid value for key "port-speeds" (set in nodes.r1)`,
		},
		"unknown-field": {
			yaml:    "- port: 2\n  speed: 1000\n  duplex: half\n",
			errText: "field duplex not found",
		},
	}

	registry := clabnodes.NewNodeRegistry()
	Register(registry)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var value any
			if err := yaml.Unmarshal([]byte(tc.yaml), &value); err != nil {
				t.Fatal(err)
			}
			got, err := clabnodes.DecodeKindSpecificConfig(
				registry.Kind("mikrotik_chr"),
				"r1",
				"mikrotik_chr",
				[]clabtypes.KindSpecificConfigEntry{
					{Key: "port-speeds", Value: value, From: "nodes.r1"},
				},
			)
			if tc.errText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errText) {
					t.Fatalf("decode = %v, want an error containing %q", err, tc.errText)
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
			cfg, ok := got.(*KindSpecificConfig)
			if !ok {
				t.Fatalf("decoded %T, want *KindSpecificConfig", got)
			}
			if len(cfg.PortSpeeds) != len(tc.want) {
				t.Fatalf("PortSpeeds = %v, want %v", cfg.PortSpeeds, tc.want)
			}
			for i := range tc.want {
				if cfg.PortSpeeds[i] != tc.want[i] {
					t.Errorf("PortSpeeds[%d] = %v, want %v", i, cfg.PortSpeeds[i], tc.want[i])
				}
			}
		})
	}
}

func TestInitPortSpeeds(t *testing.T) {
	t.Run("writes-the-launcher-variable", func(t *testing.T) {
		n := newInitialized(t, &clabtypes.NodeConfig{
			ShortName: "r1",
			LabDir:    t.TempDir(),
			KindSpecificConfig: &KindSpecificConfig{PortSpeeds: []PortSpeed{
				{Port: "2..3", Speed: 1000},
				{Port: "4", Speed: 10000},
			}},
		})
		if got, want := n.Cfg.Env[portSpeedsEnv], "2:1000 3:1000 4:10000"; got != want {
			t.Errorf("Env[%s] = %q, want %q", portSpeedsEnv, got, want)
		}
	})

	// Only the validated key reaches QEMU: a CHR_PORT_SPEEDS set in env is replaced.
	t.Run("replaces-env", func(t *testing.T) {
		n := newInitialized(t, &clabtypes.NodeConfig{
			ShortName: "r1",
			LabDir:    t.TempDir(),
			Env:       map[string]string{portSpeedsEnv: "2:1G"},
		})
		if got := n.Cfg.Env[portSpeedsEnv]; got != "" {
			t.Errorf("Env[%s] = %q, want it empty without port-speeds", portSpeedsEnv, got)
		}
	})

	t.Run("rejects-invalid", func(t *testing.T) {
		err := new(mikrotikCHR).Init(&clabtypes.NodeConfig{
			ShortName:   "r1",
			LabDir:      t.TempDir(),
			Credentials: clabtypes.NodeCredentials{Username: "admin", Password: "admin"},
			KindSpecificConfig: &KindSpecificConfig{
				PortSpeeds: []PortSpeed{{Port: "1", Speed: 1000}},
			},
		})
		if err == nil || !strings.Contains(err.Error(), `port-speeds: port "1"`) {
			t.Fatalf("Init = %v, want the port-speeds error", err)
		}
	})
}

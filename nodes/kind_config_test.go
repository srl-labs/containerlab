package nodes

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// testMode validates its value while decoding, like the SR OS config mode.
type testMode string

func (m *testMode) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	if s != "a" && s != "b" {
		return errors.New("invalid mode " + s)
	}

	*m = testMode(s)

	return nil
}

type testKindConfig struct {
	Mode   testMode `yaml:"mode,omitempty"`
	Offset *int     `yaml:"offset,omitempty"`
}

var testSpec KindConfigSpec[testKindConfig]

func testEntry(withConfig bool) *NodeRegistryEntry {
	attrs := NewNodeRegistryEntryAttributes(nil, nil, nil)
	if withConfig {
		attrs.WithKindConfig(testSpec)
	}

	return &NodeRegistryEntry{attributes: attrs}
}

func TestDecodeKindConfig(t *testing.T) {
	tests := map[string]struct {
		entry   *NodeRegistryEntry
		entries []clabtypes.KindConfigEntry
		want    any
		wantErr []string
	}{
		"decodes": {
			entry: testEntry(true),
			entries: []clabtypes.KindConfigEntry{
				{Key: "mode", Value: "a", From: "kinds.test"},
				{Key: "offset", Value: 3, From: "nodes.n1"},
			},
			want: &testKindConfig{Mode: "a", Offset: new(3)},
		},
		"no_keys_zero_config": {
			entry: testEntry(true),
			want:  &testKindConfig{},
		},
		"kind_without_config_no_keys": {
			entry: testEntry(false),
		},
		"unknown_keys_and_invalid_values_reported_together": {
			entry: testEntry(true),
			entries: []clabtypes.KindConfigEntry{
				{Key: "mode", Value: "c", From: "defaults"},
				{Key: "modee", Value: "a", From: "groups.g"},
				{Key: "offset", Value: "abc", From: "nodes.n1"},
			},
			wantErr: []string{
				`node "n1": invalid value for key "mode" (set in defaults): invalid mode c`,
				`node "n1": kind "test" does not support key "modee" (set in groups.g)`,
				`node "n1": invalid value for key "offset" (set in nodes.n1): cannot unmarshal !!str ` + "`abc`" + ` into int`,
			},
		},
		"kind_without_config": {
			entry:   testEntry(false),
			entries: []clabtypes.KindConfigEntry{{Key: "mode", Value: "a", From: "defaults"}},
			wantErr: []string{
				`node "n1": kind "test" does not support key "mode" (set in defaults)`,
			},
		},
		"unregistered_kind": {
			entries: []clabtypes.KindConfigEntry{{Key: "mode", Value: "a", From: "nodes.n1"}},
			wantErr: []string{`kind "test" does not support key "mode"`},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := DecodeKindConfig(tc.entry, "n1", "test", tc.entries)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				}
				for _, want := range tc.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Fatalf("kind config mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestAcceptsKindConfigKey(t *testing.T) {
	e := testEntry(true)

	for key, want := range map[string]bool{"mode": true, "offset": true, "modee": false} {
		if got := e.AcceptsKindConfigKey(key); got != want {
			t.Errorf("AcceptsKindConfigKey(%q) = %v, want %v", key, got, want)
		}
	}

	if testEntry(false).AcceptsKindConfigKey("mode") {
		t.Error("kind without config accepts a key")
	}
}

func TestKindConfigSpecOf(t *testing.T) {
	cfg := &clabtypes.NodeConfig{}

	testSpec.Of(cfg).Mode = "a"

	if got := testSpec.Of(cfg); got.Mode != "a" || cfg.KindConfig != any(got) {
		t.Fatalf("Of did not keep the zero value it set on cfg, got %+v", got)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("Of on another kind's config did not panic")
		}
	}()

	KindConfigSpec[struct{}]{}.Of(cfg)
}

type defaultedKindConfig struct {
	Enabled bool `yaml:"enabled"`
}

func (c *defaultedKindConfig) SetDefaults() { c.Enabled = true }

func TestKindConfigDefaults(t *testing.T) {
	var spec KindConfigSpec[defaultedKindConfig]

	if !spec.Of(&clabtypes.NodeConfig{}).Enabled {
		t.Error("Of on an empty node config did not apply defaults")
	}

	e := &NodeRegistryEntry{
		attributes: NewNodeRegistryEntryAttributes(nil, nil, nil).WithKindConfig(spec),
	}

	got, err := DecodeKindConfig(e, "n1", "test", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !got.(*defaultedKindConfig).Enabled {
		t.Error("decode without keys did not apply defaults")
	}

	got, err = DecodeKindConfig(e, "n1", "test",
		[]clabtypes.KindConfigEntry{{Key: "enabled", Value: false, From: "nodes.n1"}})
	if err != nil {
		t.Fatal(err)
	}

	if got.(*defaultedKindConfig).Enabled {
		t.Error("explicit false did not override the default")
	}
}

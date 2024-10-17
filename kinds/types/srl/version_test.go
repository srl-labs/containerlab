package srl

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVersionString(t *testing.T) {
	n := &srl{}

	tests := map[string]struct {
		s    string
		want *SrlVersion
	}{
		"valid version string": {
			s:    "v24.3.1-154-gffc27e28d7",
			want: &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
		},
		"valid beta version string": {
			s:    "v24.7.1-330-gffc27e28d7-dirty",
			want: &SrlVersion{"24", "7", "1", "330", "gffc27e28d7-dirty"},
		},
		"invalid version string": {
			s:    "invalid",
			want: &SrlVersion{"0", "0", "0", "0", "0"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := n.parseVersionString(tt.s)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatalf("parseVersionString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSrlVersionString(t *testing.T) {
	tests := map[string]struct {
		v    *SrlVersion
		want string
	}{
		"all fields filled": {
			v:    &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
			want: "v24.3.1-154-gffc27e28d7",
		},
		"all fields empty": {
			v:    &SrlVersion{"0", "0", "0", "0", "0"},
			want: "v0.0.0-0-0",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("SrlVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMajorMinorSemverString(t *testing.T) {
	tests := map[string]struct {
		v    *SrlVersion
		want string
	}{
		"all fields filled": {
			v:    &SrlVersion{"24", "3", "1", "154", "gffc27e28d7"},
			want: "v24.3",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.v.MajorMinorSemverString(); got != tt.want {
				t.Errorf("SrlVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

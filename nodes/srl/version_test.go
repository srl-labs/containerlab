package srl

import (
	"reflect"
	"testing"
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
		"invalid version string": {
			s:    "invalid",
			want: &SrlVersion{"0", "0", "0", "0", "0"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := n.parseVersionString(tt.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseVersionString() = %v, want %v", got, tt.want)
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

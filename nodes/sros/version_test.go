package sros

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVersionString(t *testing.T) {
	n := &sros{}

	tests := []struct {
		name string
		s    string
		want *SrosVersion
	}{
		{
			name: "valid version string",
			s:    "v25.3.R1",
			want: &SrosVersion{"25", "3", "R1"},
		},
		{
			name: "valid beta version string",
			s:    "v24.7.1-330-gffc27e28d7-dirty",
			want: &SrosVersion{"24", "7", "1"},
		},
		{
			name: "invalid version string",
			s:    "invalid",
			want: &SrosVersion{"0", "0", "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := n.parseVersionString(tt.s)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatalf("parseVersionString() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSrosVersionString(t *testing.T) {
	tests := []struct {
		name string
		v    *SrosVersion
		want string
	}{
		{
			name: "all fields filled",
			v:    &SrosVersion{"24", "3", "1"},
			want: "v24.3.1",
		},
		{
			name: "all fields empty",
			v:    &SrosVersion{"0", "0", "0"},
			want: "v0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("SrosVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMajorMinorSemverString(t *testing.T) {
	tests := []struct {
		name string
		v    *SrosVersion
		want string
	}{
		{
			name: "all fields filled",
			v:    &SrosVersion{"24", "3", "1"},
			want: "v24.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.MajorMinorSemverString(); got != tt.want {
				t.Errorf("SrosVersion.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

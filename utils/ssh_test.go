package utils

import (
	"testing"
)

func TestParseSSHVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "valid version",
			in:   "OpenSSH_8.1p1 Debian-8, OpenSSL 1.1.1d  10 Sep 2019",
			want: "8.1",
		},
		{
			name: "another valid version",
			in:   "OpenSSH_8.9p1 Ubuntu-3ubuntu0.3, OpenSSL 3.0.2 15 Mar 2022",
			want: "8.9",
		},
		{
			name: "invalid version",
			in:   "Invalid version string",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSSHVersion(tt.in)
			if got != tt.want {
				t.Errorf("parseSSHVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

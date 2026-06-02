package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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

func Test_MarshalAndCatenateSSHPubKeys(t *testing.T) {
	type fields struct {
		keyFiles []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test1",
			fields: fields{
				keyFiles: []string{"test_data/keys"},
			},
			want: "\"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCs4Qv1yrBk6ygt+o7J4sUcYv+WfDjdAyABDoinOt3PgSmCcVqqAP2qS8UtTnMNuy93Orp6+/R/7/R3O5xdY6I4YViK3WVlKTAUVm7vdeTKp9uq1tNeWgo7+J3baSbQ3INp85ScTfFvRzRCFkr/W97Wh6pTa7ysgkcPvc2/tXG2z36Mx7/TFBk3Q1LY3ByKLtGrC5JnVpMTrqrsCwcLEVHHEZ4z5R4FZED/lpz+wTNFnR/l9HA6yDkKYensHynx+guqYpYD6y4yEGY/LcUnwBg0zIlUhmOsvdmxWBz12Lp7EBiNjSwhnPfe+o3efLGGnjWUAa4TgO8Sa8PQP0pK/ZNd\" \"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILKdXYzPIq8kHRJtDrh21wMVI76AnuPk7HDLeDteKN74\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := LoadSSHPubKeysFromFiles(tt.fields.keyFiles)
			if err != nil {
				t.Errorf("failed to load keys: %v", err)
			}

			got := MarshalAndCatenateSSHPubKeys(keys)

			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("MarshalAndCatenateSSHPubKeys() = %s", d)
			}
		})
	}
}

package srl

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/srl-labs/containerlab/utils"
)

func Test_srl_catenateKeys(t *testing.T) {
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
			keys, err := utils.LoadSSHPubKeysFromFiles(tt.fields.keyFiles)
			if err != nil {
				t.Errorf("failed to load keys: %v", err)
			}

			n := &srl{
				sshPubKeys: keys,
			}

			got := catenateKeys(n.sshPubKeys)

			if d := cmp.Diff(got, tt.want); d != "" {
				t.Errorf("srl.catenateKeys() = %s", d)
			}
		})
	}
}

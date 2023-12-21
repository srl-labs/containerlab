package vr_sros

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func Test_vrSROS_mapSSHPubKeys(t *testing.T) {
	tests := []struct {
		name          string
		keysAsStr     [][]byte
		expectedRSA   int
		expectedECDSA int
	}{
		{
			name: "Test with 3 rsa and 4 ecdsa keys",
			keysAsStr: [][]byte{
				[]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDhR6+tNYAAP0i40O5INIrSGMuhQH2R6nsH0ZfPlMScmvORGgGPyFFHg76MiVIudEnwDKaytkZZmOJM1aWvF94WZJQDhzR6Uz9W8k14cRDPhOrLXjTbu2LKWRJtJpvcVB/gjhMoOuzGID/lBzpzC4bKLVYBze7Ek3XrTjf6xlB3I4G3GVCJHO6gDfpQqSPucXiXtO2gMhk1sKDVDD1N/31VSxVwHDB1BHUREM/rbmGHSSJaEjAByxYIL80kLQRTejnL3NMXGbuVisO6OL4H2h9gdTNppjQjSUCKcn/IFOx7LuYO64xGm3ThfBKRWVQNul2Ab/pL4/BqD4ziZ3Wad94Ob7kClKeY9rO9I1wfSckDaF1CvabeyM63ekNF8xOViDOfBJGK+eOWsN5nKVcD2lSzQZAolDGSYQPcSeZ82/oqcHrG/kUcVZxuU5Lkc8QpDYlW0EorrUIvgM0Ta7GrmcxAU1WxUD9W6GzNbrCm71eP4Gu9ZD+U6p0+r4XC0bGUThE="),
				[]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1XraioeGL9OVkGVu9yMLVkVWMYrp2yP377HoKqA63VuDuJMDHVKT/BDINh/NUiorS1jOwu3cE66ZvU8w2hC9gkIdCUZ6ZTsUP2Zb1bk1V/kNbjd77Ra9iSssK/o2SBzeOZ6k0xbz1Yp9cXbTNh2s5g5/E764rd2hdyRWYagC1re7hrTBuxEXEJC4+1iUsp2gC1ZxNG2ol1UOk1e7sHV/eiw9VyIDyNiU0d8Dul4a5/Fp4/vBiEI6BvVegeQTgJd29Guf2Tl7oOwFiQpdDevnN/CcsCuEF5zcdywIujaLwwEIjI7rf6210DXQdxNlruI1Oq8koIn/WAhH/geeRDA7mSIRxYF2Uqb+VErNF2iTq7P6/609bcXow8S5TfB8PIQH/E9gItTg2XlgAwAB/HYlUUUwrjhNYUtYvQq3a+EjdJFKTJdHQp08xVb1f4aD+61NSROwtwHm+NHttdSIo9nqqSpdm0KxlS9t6iVVJALqy8oiUEBWJDVilRB7RLyHt0sk="),
				[]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDXvF3x+IWYfy957yhPqN+kfO/vCXitJ4jAZJVp28P5yw43TKHFqmoecITEtS4i8Rrt+w+L6I7t4XKRptEeISLdOzxkMgVfutShqBrxMkWSmwRd2vFABT5In1gsU666cahAF2n7kNC2X2OBw8oANcsk+nsPGZzOfFyCPB1Vc8z/66UJAbe3bwSjnwzUAShXkPpMmaGhm7+LY9foH4aW0ho4vlGXnZ7M1WDlnBC+9ae8MUP5eAgNw+q0MPlKmzDXDoxQTKEtY1Mu66oacUmIsqxRFajrVCplc2O9MGhcNw8Z+TKkrP7cJxgX0kMh92m2LVV3TLMcI513aBhvDzwHdTH/Tqn/qpa2Jpqovg9RpJBCIjX8tN6j8TKsdfDnqadW8HL89Lrlv36lk6DOD872z3zxajH9FaJgVjQjxEVYafZDqgHHNnNLp05IBwuExV2q93ML7WdhWemSgIVV2GF7cgEcnHWm+Q7JKCJTMEUBimshJEaMm6ROcgfDO4KFpMYpJAc="),
				[]byte("ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBBiXo8DavZ3rzNfynQCr5WJia1S5VT2Jx91OEjqeipcwzpqqeSciaIIwGt78Oq7PCNNrBoYDns+rjtzKq4ViFhk="),
				[]byte("ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBDi9AlTU5OMFKIjLSCQMCwH+IQtv82sfU2BDsSLdFeM7xiZtNSUzoaH+WQA61rwNi4sc3iddQLv54LyxC9Z7auL+ItsoDulQ9TklBq7mlmQfS5yS0LsdSAaK7iGKfiLong=="),
				[]byte("ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBABZMizDux+9W92bOxwgPVTTau4xLzcUVF+vfkySVaop7cq8YtJ4QbxTbOkawpG8ZC9gCGzhiVqF7aFhoMIF0jpF5AGcBUxm1ahp7uURmI7YXjlnvzHMgrp0ot8sdY8ibjZSGYzK0BuGOsZFq1cFQRJjIWoNTJjqRFqA/gLIWaab2V9nrg=="),
				[]byte("ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBAC+ZByJkRXxaZQQ3NDJRDQudtrV1PguOUJ/43UFMmHr8KfA5+oYG/lQzFgXwXxe4OWefJPjuSEqcNuw3xvb5MTVJwFEyXPRyyjljSUhdocWemAMb9oh8t9B4daPwoe0sVG7m9VH0h4uESFuf/SjHhngZgtuh22j9OpEbMiJjOgGL3JC/w=="),
			},
			expectedRSA:   3,
			expectedECDSA: 4,
		},
		{
			name:          "Test without keys",
			keysAsStr:     [][]byte{},
			expectedRSA:   0,
			expectedECDSA: 0,
		},
		{
			name: "Test with 35 RSA and 40 ecdsa keys",
			keysAsStr: func() [][]byte {
				rsaKey := []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDXvF3x+IWYfy957yhPqN+kfO/vCXitJ4jAZJVp28P5yw43TKHFqmoecITEtS4i8Rrt+w+L6I7t4XKRptEeISLdOzxkMgVfutShqBrxMkWSmwRd2vFABT5In1gsU666cahAF2n7kNC2X2OBw8oANcsk+nsPGZzOfFyCPB1Vc8z/66UJAbe3bwSjnwzUAShXkPpMmaGhm7+LY9foH4aW0ho4vlGXnZ7M1WDlnBC+9ae8MUP5eAgNw+q0MPlKmzDXDoxQTKEtY1Mu66oacUmIsqxRFajrVCplc2O9MGhcNw8Z+TKkrP7cJxgX0kMh92m2LVV3TLMcI513aBhvDzwHdTH/Tqn/qpa2Jpqovg9RpJBCIjX8tN6j8TKsdfDnqadW8HL89Lrlv36lk6DOD872z3zxajH9FaJgVjQjxEVYafZDqgHHNnNLp05IBwuExV2q93ML7WdhWemSgIVV2GF7cgEcnHWm+Q7JKCJTMEUBimshJEaMm6ROcgfDO4KFpMYpJAc=")

				ecdsaKey := []byte("ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBAC+ZByJkRXxaZQQ3NDJRDQudtrV1PguOUJ/43UFMmHr8KfA5+oYG/lQzFgXwXxe4OWefJPjuSEqcNuw3xvb5MTVJwFEyXPRyyjljSUhdocWemAMb9oh8t9B4daPwoe0sVG7m9VH0h4uESFuf/SjHhngZgtuh22j9OpEbMiJjOgGL3JC/w==")

				keys := make([][]byte, 75)
				for i := 0; i < 35; i++ {
					keys[i] = rsaKey
				}
				for i := 35; i < 75; i++ {
					keys[i] = ecdsaKey
				}

				return keys
			}(),
			expectedRSA:   32, // SROS supports a maximum of 32 keys per key type
			expectedECDSA: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := make([]ssh.PublicKey, 0, len(tt.keysAsStr))
			for _, strKey := range tt.keysAsStr {
				key, _, _, _, err := ssh.ParseAuthorizedKey(strKey)
				if err != nil {
					t.Error(err)
				}
				keys = append(keys, key)
			}

			s := &vrSROS{
				sshPubKeys: keys,
			}

			tmplData := &SROSTemplateData{}

			s.prepareSSHPubKeys(tmplData)

			if len(tmplData.SSHPubKeysRSA) != tt.expectedRSA {
				t.Errorf("expected %d RSA keys, got %d", tt.expectedRSA, len(tmplData.SSHPubKeysRSA))
			}

			if len(tmplData.SSHPubKeysECDSA) != tt.expectedECDSA {
				t.Errorf("expected %d ECDSA keys, got %d", tt.expectedECDSA, len(tmplData.SSHPubKeysECDSA))
			}
		})
	}
}

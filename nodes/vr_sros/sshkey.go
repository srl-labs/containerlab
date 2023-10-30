package vr_sros

import (
	"slices"
	"strings"

	"golang.org/x/crypto/ssh"
)

// filterSSHPubKeys removes non-rsa keys from n.sshPubKeys until srl adds support for them.
func (s *vrSROS) filterSSHPubKeys(sshKeyAlgo []string) []string {

	keyValues := []string{}

	for _, k := range s.sshPubKeys {
		if slices.Contains(sshKeyAlgo, k.Type()) {
			//switch k.Type() {
			//case slices.Contains(sshKeyAlgo):

			keyType := k.Type()
			keyString := string(ssh.MarshalAuthorizedKey(k))

			// Remove the key type prefix
			keyString = strings.TrimPrefix(keyString, keyType+" ")

			// Remove the suffix (usually a comment or username)
			parts := strings.Fields(keyString)
			if len(parts) > 1 {
				keyString = parts[0]
			}

			keyValues = append(keyValues, keyString)

		}
	}

	return keyValues
}
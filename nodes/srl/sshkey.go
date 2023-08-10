package srl

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// catenateKeys catenates the ssh public keys
// and produces a string that can be used in the
// cli config command to set the ssh public keys
// for users.
func catenateKeys(in []ssh.PublicKey) string {
	var keys string

	for i, k := range in {
		// marshall the publickey in authorizedKeys format
		// and trim spaces (cause there will be a trailing newline)
		ks := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
		// catenate all ssh keys into a single quoted string accepted in CLI
		keys += fmt.Sprintf("%q", ks)
		// only add a space after the key if it is not the last one
		if i < len(in)-1 {
			keys += " "
		}
	}

	return keys
}

// filterSSHPubKeys removes non-rsa keys from n.sshPubKeys until srl adds support for them.
func (n *srl) filterSSHPubKeys() {
	filteredKeys := []ssh.PublicKey{}

	for _, k := range n.sshPubKeys {
		if k.Type() == "ssh-rsa" {
			filteredKeys = append(filteredKeys, k)
		}
	}

	n.sshPubKeys = filteredKeys
}

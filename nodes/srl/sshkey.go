package srl

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// catenateKeys catenates the ssh public keys
// and produces a string that can be used in the
// cli config command to set the ssh public keys
// for users.
func catenateKeys(in []ssh.PublicKey) string {
	var keys strings.Builder
	// iterate through keys
	for _, k := range in {
		// extract the keys in AuthorizedKeys format (e.g. "ssh-rsa <KEY>")
		ks := bytes.TrimSpace(ssh.MarshalAuthorizedKey(k))
		// add a seperator, leading quote, the key string and trailing quote
		fmt.Fprintf(&keys, " \"%s\"", ks)
	}
	// return all but the first leading seperator of the string builders content as string
	return keys.String()[1:]
}

// filterSSHPubKeys removes non-rsa keys from n.sshPubKeys until srl adds support for them.
func (n *srl) filterSSHPubKeys() {
	var filteredKeys []ssh.PublicKey

	for _, k := range n.sshPubKeys {
		if k.Type() == ssh.KeyAlgoRSA {
			filteredKeys = append(filteredKeys, k)
		}
	}

	n.sshPubKeys = filteredKeys
}

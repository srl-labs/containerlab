package srl

import (
	"bytes"
	"strings"

	"golang.org/x/crypto/ssh"
)

// catenateKeys catenates the ssh public keys
// and produces a string that can be used in the
// cli config command to set the ssh public keys
// for users.
func catenateKeys(in []ssh.PublicKey) string {
	var keys strings.Builder
	// pre-allocate the string builder capacity
	keys.Grow(len(in) * 100)
	// iterate through keys
	for i, k := range in {
		// extract the keys in AuthorizedKeys format (e.g. "ssh-rsa <KEY>")
		ks := bytes.TrimSpace(ssh.MarshalAuthorizedKey(k))
		// add a separator, leading quote, the key string and trailing quote
		if i > 0 {
			keys.WriteByte(' ')
		}
		keys.WriteByte('"')
		keys.Write(ks)
		keys.WriteByte('"')
	}
	// return the string builders content as string
	return keys.String()
}

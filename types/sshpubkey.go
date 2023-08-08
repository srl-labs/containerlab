package types

import (
	"bytes"

	"golang.org/x/crypto/ssh"
)

type SSHPubKey struct {
	PublicKey ssh.PublicKey
	Comment   string
}

func NewSSHPublicKey(pubkey ssh.PublicKey, comment string) *SSHPubKey {
	return &SSHPubKey{
		PublicKey: pubkey,
		Comment:   comment,
	}
}

func (s *SSHPubKey) Equals(s2 *SSHPubKey) bool {
	return bytes.Equal(s.PublicKey.Marshal(), s2.PublicKey.Marshal())
}

package types

import "golang.org/x/crypto/ssh"

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

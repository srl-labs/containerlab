package types

type PubkeyAuthValue string

const (
	PubkeyAuthValueYes       PubkeyAuthValue = "yes"
	PubkeyAuthValueNo        PubkeyAuthValue = "no"
	PubkeyAuthValueHostBound PubkeyAuthValue = "host-bound"
	PubkeyAuthValueUnbound   PubkeyAuthValue = "unbound"
)

func (p PubkeyAuthValue) String() string {
	return string(p)
}

// SSHConfig is the SSH client configuration that a clab node requires.
type SSHConfig struct {
	PubkeyAuthentication PubkeyAuthValue
}

func NewSSHConfig() *SSHConfig {
	return &SSHConfig{}
}

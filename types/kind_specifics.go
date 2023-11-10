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

type KindSpecifics struct {
	SSHSpecifics *SSHSpecifics
}

func NewKindSpecifics() *KindSpecifics {
	return &KindSpecifics{
		SSHSpecifics: NewSSHSpecifics(),
	}
}

type SSHSpecifics struct {
	PubkeyAuthentication PubkeyAuthValue
}

func NewSSHSpecifics() *SSHSpecifics {
	return &SSHSpecifics{}
}

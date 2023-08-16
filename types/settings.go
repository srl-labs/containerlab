package types

import "time"

type Settings struct {
	CertificateAuthority *CertificateAuthority `yaml:"certificate-authority"`
}

type CertificateAuthority struct {
	Cert             string        `yaml:"cert"`
	Key              string        `yaml:"key"`
	KeySize          int           `yaml:"key-size"`
	ValidityDuration time.Duration `yaml:"validity-duration"`
}

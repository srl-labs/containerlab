package types

import "time"

// Settings is the structure for global containerlab settings.
type Settings struct {
	CertificateAuthority *CertificateAuthority `yaml:"certificate-authority"`
}

// CertificateAuthority is the structure for global containerlab certificate authority settings.
type CertificateAuthority struct {
	// Cert is the path to the CA certificate file in the External CA mode of operation.
	Cert string `yaml:"cert"`
	// Key is the path to the CA private key file in the External CA mode of operation.
	Key string `yaml:"key"`
	// KeySize is the size of the CA private key in bits
	// when containerlab is in charge of the CA generation.
	KeySize int `yaml:"key-size"`
	// ValidityDuration is the duration of the CA certificate validity
	// when containerlab is in charge of the CA generation.
	ValidityDuration time.Duration `yaml:"validity-duration"`
}

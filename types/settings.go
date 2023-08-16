package types

type Settings struct {
	CertificateAuthority *CertificateAuthority `yaml:"certificate-authority"`
}

type CertificateAuthority struct {
	Cert             string `yaml:"cert"`
	Key              string `yaml:"key"`
	KeySize          int    `yaml:"key-size"`
	ValidityDuration string `yaml:"validity-duration"`
}

type LinkConfig struct {
	Endpoints []string
	Labels    map[string]string      `yaml:"labels,omitempty"`
	Vars      map[string]interface{} `yaml:"vars,omitempty"`
	MTU       int                    `yaml:"mtu,omitempty"`
}

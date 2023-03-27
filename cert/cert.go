package cert

// Cert is a wrapper struct for the Certificate Authority and the Certificate Storage interfaces.
type Cert struct {
	CA
	CertStorage
}

// CA is an interface that wraps methods needed to generate CA and Node certificates.
type CA interface {
	// SetCACert sets CA Certificate to the CertificateAuthority implementation
	SetCACert(cert *Certificate) error
	// GenerateCACert generates a CA certificate, key and CSR based on the provided input.
	GenerateCACert(input *CACSRInput) (*Certificate, error)
	// GenerateAndSignNodeCert generates and signs a node certificate, key and CSR based on the provided input and signs it with the CA.
	GenerateAndSignNodeCert(input *NodeCSRInput) (*Certificate, error)
}

// CertStorage is an interface that wraps methods to load and store certificates.
type CertStorage interface {
	LoadCaCert() (*Certificate, error)
	LoadNodeCert(nodeName string) (*Certificate, error)
	StoreCaCert(cert *Certificate) error
	StoreNodeCert(nodeName string, cert *Certificate) error
}

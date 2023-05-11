package cert

// Cert is a wrapper struct for the Certificate Authority and the Certificate Storage.
type Cert struct {
	*CA
	CertStorage
}

// CertStorage is an interface that wraps methods to load and store certificates.
type CertStorage interface {
	LoadCaCert() (*Certificate, error)
	LoadNodeCert(nodeName string) (*Certificate, error)
	StoreCaCert(cert *Certificate) error
	StoreNodeCert(nodeName string, cert *Certificate) error
}

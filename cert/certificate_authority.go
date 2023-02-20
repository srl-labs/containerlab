package cert

// CertificateAuthority is the interface satisfied by the CertificateAuthority implementation.
// it is used to generate root certificates as well as node based certificates signed by the root ca.
type CertificateAuthority interface {
	// SetCACert sets CA Certificate to the CertificateAuthority implementation
	SetCACert(cert *Certificate) error
	// GenerateCACert generates a CA certificate, key and CSR based on the provided input.
	GenerateCACert(input *CACSRInput) (*Certificate, error)
	// GenerateNodeCert generates a node certificate, key and CSR based on the provided input and signs it with the CA.
	GenerateNodeCert(input *NodeCSRInput) (*Certificate, error)
}

package cert

// CertificateAuthority is the interface satisfied by the CertificateAuthority implementation.
// it is used to generate root certificates as well as node based certificates signed by the root ca
type CertificateAuthority interface {
	// SetRootCertificate provides the externally loaded Root Certificate (Cert and Key) to the CertificateAuthority implementation
	SetRootCertificate(cert *Certificate) error
	// GenerateRootCert will make the CertificateAuthority generate a new Root CA certificate, init the internal state, such that node
	// certs can be generated and return the Root-CA Certificate (Cert, Key and CSR)
	GenerateRootCert(input *CACSRInput) (*Certificate, error)
	// GenerateNodeCert requests a new Node certificate from the CertificateAuthority, which will generate the Certificates and will sign those
	// with the already setup Root CA cert.
	GenerateNodeCert(input *NodeCSRInput) (*Certificate, error)
}

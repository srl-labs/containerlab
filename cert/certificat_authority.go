package cert

type CertificateAuthority interface {
	SetRootCertificate(cert *Certificate) error
	GenerateRootCert(input *CaCertInput) (*Certificate, error)
	GenerateCert(input *NodeCertInput) (*Certificate, error)
}

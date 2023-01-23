package cert

// NodeCertInput struct.
type NodeCertInput struct {
	Hosts            []string
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           string

	Name     string
	LongName string
	Fqdn     string
	SANs     []string
	Prefix   string
}

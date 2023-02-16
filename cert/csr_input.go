package cert

// CACSRInput struct.
type CACSRInput struct {
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           string

	Prefix string
}

// NodeCSRInput struct.
type NodeCSRInput struct {
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

package cert

// CACSRInput struct.
type CACSRInput struct {
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           string
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
}

package cert

import "time"

// CACSRInput struct.
type CACSRInput struct {
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           time.Duration
	KeySize          int
}

// NodeCSRInput struct.
type NodeCSRInput struct {
	Hosts            []string
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           time.Duration
	KeySize          int
}

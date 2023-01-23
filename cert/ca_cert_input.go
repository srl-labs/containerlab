package cert

// CaCertInput struct.
type CaCertInput struct {
	CommonName       string
	Country          string
	Locality         string
	Organization     string
	OrganizationUnit string
	Expiry           string

	Prefix string
	Names  map[string]string // Not used right now
}

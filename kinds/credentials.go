package kinds

// Credentials defines NOS SSH credentials.
type Credentials struct {
	username string
	password string
}

// NewCredentials constructor for the Credentials struct.
func NewCredentials(username, password string) *Credentials {
	return &Credentials{
		username: username,
		password: password,
	}
}

func (c *Credentials) GetUsername() string {
	if c == nil {
		return ""
	}

	return c.username
}

func (c *Credentials) GetPassword() string {
	if c == nil {
		return ""
	}

	return c.password
}

// Slice returns credentials as a slice.
func (c *Credentials) Slice() []string {
	if c == nil {
		return nil
	}

	return []string{c.GetUsername(), c.GetPassword()}
}

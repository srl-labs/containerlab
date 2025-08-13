package core

import (
	"fmt"

	clabcert "github.com/srl-labs/containerlab/cert"
)

// LoadOrGenerateCA loads the CA certificate from the storage, or generates a new one if it does not exist.
func (c *CLab) LoadOrGenerateCA(caCertInput *clabcert.CACSRInput) error {
	// try loading the CA cert, and if it fails, generate a new one
	caCertificate, err := c.Cert.LoadCaCert()
	if err != nil {
		// if loading certs failed, try to generate new RootCA
		caCertificate, err = c.Cert.GenerateCACert(caCertInput)
		if err != nil {
			return fmt.Errorf("failed generating new Root CA %v", err)
		}
		// store the root CA
		err = c.Cert.StoreCaCert(caCertificate)
		if err != nil {
			return nil
		}
	}

	// set CA cert that was either loaded or generated
	err = c.Cert.SetCACert(caCertificate)
	if err != nil {
		return nil
	}

	return nil
}

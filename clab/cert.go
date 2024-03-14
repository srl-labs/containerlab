package clab

import (
	"fmt"

	"github.com/srl-labs/containerlab/cert"
)

// loadOrGenerateCA loads the CA certificate from the storage, or generates a new one if it does not exist.
func (c *CLab) loadOrGenerateCA(caCertInput *cert.CACSRInput) error {
	// try loading the CA cert, and if it fails, generate a new one
	caCertificate, err := c.cert.LoadCaCert()
	if err != nil {
		// if loading certs failed, try to generate new RootCA
		caCertificate, err = c.cert.GenerateCACert(caCertInput)
		if err != nil {
			return fmt.Errorf("failed generating new Root CA %v", err)
		}
		// store the root CA
		err = c.cert.StoreCaCert(caCertificate)
		if err != nil {
			return nil
		}
	}

	// set CA cert that was either loaded or generated
	err = c.cert.SetCACert(caCertificate)
	if err != nil {
		return nil
	}

	return nil
}

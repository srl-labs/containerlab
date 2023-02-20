package clab

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/cert"
)

// CertificateAuthority represents a Certificate Authority.
type CertificateAuthority interface {
	// SetCACert sets CA Certificate to the CertificateAuthority implementation
	SetCACert(cert *cert.Certificate) error
	// GenerateCACert generates a CA certificate, key and CSR based on the provided input.
	GenerateCACert(input *cert.CACSRInput) (*cert.Certificate, error)
	// GenerateNodeCert generates a node certificate, key and CSR based on the provided input and signs it with the CA.
	GenerateNodeCert(input *cert.NodeCSRInput) (*cert.Certificate, error)
}

// CertStorage defines the interface used to manage certificate storage.
type CertStorage interface {
	LoadCaCert() (*cert.Certificate, error)
	LoadNodeCert(nodeName string) (*cert.Certificate, error)
	StoreCaCert(cert *cert.Certificate) error
	StoreNodeCert(nodeName string, cert *cert.Certificate) error
}

// LoadOrGenerateCA loads the CA certificate from the storage, or generates a new one if it does not exist.
func (c *CLab) LoadOrGenerateCA(caCertInput *cert.CACSRInput) error {
	// try loading the CA cert, and if it fails, generate a new one
	caCertificate, err := c.certStorage.LoadCaCert()
	if err != nil {
		// if loading certs failed, try to generate new RootCA
		caCertificate, err = c.CA.GenerateCACert(caCertInput)
		if err != nil {
			return fmt.Errorf("failed generating new Root CA %v", err)
		}
		// store the root CA
		err = c.certStorage.StoreCaCert(caCertificate)
		if err != nil {
			return nil
		}
	}

	// set CA cert that was either loaded or generated
	err = c.CA.SetCACert(caCertificate)
	if err != nil {
		return nil
	}

	return nil
}

// GenerateMissingNodeCerts generates missing node certificates and stores them in the storage.
func (c *CLab) GenerateMissingNodeCerts() error {
	for _, n := range c.Nodes {
		nodeConfig := n.Config()

		// try loading existing certificates from disk and generate new ones if they do not exist
		_, err := c.certStorage.LoadNodeCert(nodeConfig.ShortName)
		if err != nil {
			log.Debugf("creating node certificate for %s", nodeConfig.ShortName)

			hosts := []string{
				nodeConfig.ShortName,
				nodeConfig.LongName,
				nodeConfig.ShortName + "." + c.Config.Name + ".io",
			}
			hosts = append(hosts, nodeConfig.SANs...)

			// collect cert details
			certInput := &cert.NodeCSRInput{
				CommonName:   nodeConfig.ShortName + "." + c.Config.Name + ".io",
				Hosts:        hosts,
				Organization: "containerlab",
			}
			// Generate the cert for the node
			nodeCert, err := c.CA.GenerateNodeCert(certInput)
			if err != nil {
				return err
			}

			// persist the cert via certStorage
			err = c.certStorage.StoreNodeCert(nodeConfig.ShortName, nodeCert)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

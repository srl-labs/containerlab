package cert

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

type Certificate struct {
	Key  []byte
	Csr  []byte
	Cert []byte
}

// LoadCertificateFromDisk loads a set of cert, key and possibly the existing csr file into a Certificate struct, returnung the pointer
func LoadCertificateFromDisk(certFilename, keyFilename, csrFilename string) (*Certificate, error) {
	cert := &Certificate{}

	// Cert
	_, err := os.Stat(certFilename)
	if err != nil {
		return nil, fmt.Errorf("failed loading cert file %v", err)
	}
	cert.Cert, err = utils.ReadFileContent(certFilename)
	if err != nil {
		return nil, err
	}

	// Key
	_, err = os.Stat(keyFilename)
	if err != nil {
		return nil, fmt.Errorf("failed loading key file %v", err)
	}
	cert.Key, err = utils.ReadFileContent(keyFilename)
	if err != nil {
		return nil, err
	}

	// CSR
	_, err = os.Stat(csrFilename)
	if err != nil {
		log.Debugf("failed loading csr %s, continuing anyways", csrFilename)
	} else {
		cert.Csr, err = utils.ReadFileContent(csrFilename)
		if err != nil {
			return nil, err
		}
	}

	return cert, nil
}

// Write writes the cert content to disk
func (c *Certificate) Write(certFile, keyFile, csrFile string) error {
	err := utils.CreateFile(certFile, string(c.Cert))
	if err != nil {
		return err
	}
	err = utils.CreateFile(keyFile, string(c.Key))
	if err != nil {
		return err
	}
	// try only storing the csr if its length is >0
	if len(c.Csr) != 0 && csrFile != "" {
		err = utils.CreateFile(csrFile, string(c.Csr))
		if err != nil {
			return err
		}
	}
	return nil
}

package cert

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

// Certificate stores the combination of Cert and Key along with the CSR if available.
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

// Write writes the cert, key and csr to disk.
func (c *Certificate) Write(certPath, keyPath, csrPath string) error {
	log.Debugf("writing cert file to %s", certPath)

	err := utils.CreateFile(certPath, string(c.Cert))
	if err != nil {
		return err
	}

	log.Debugf("writing key file to %s", keyPath)
	err = utils.CreateFile(keyPath, string(c.Key))
	if err != nil {
		return err
	}

	// save csr if its length is >0 and path is not empty
	if len(c.Csr) != 0 && csrPath != "" {
		log.Debugf("writing csr file to %s", csrPath)

		err = utils.CreateFile(csrPath, string(c.Csr))
		if err != nil {
			return err
		}
	}

	return nil
}

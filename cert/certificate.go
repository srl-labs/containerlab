package cert

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// Certificate stores the combination of Cert and Key along with the CSR if available.
type Certificate struct {
	Cert []byte
	Key  []byte
	Csr  []byte
}

// NewCertificateFromFile creates a new Certificate by loading cert, key and csr (if exists) from respecting files.
func NewCertificateFromFile(certFilePath, keyFilePath, csrFilePath string) (*Certificate, error) {
	cert := &Certificate{}

	// Cert
	_, err := os.Stat(certFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed loading cert file %v", err)
	}
	cert.Cert, err = clabutils.ReadFileContent(certFilePath)
	if err != nil {
		return nil, err
	}

	// Key
	_, err = os.Stat(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed loading key file %v", err)
	}
	cert.Key, err = clabutils.ReadFileContent(keyFilePath)
	if err != nil {
		return nil, err
	}

	// CSR
	// The CSR might not be there, which is not an issue, just skip it
	if csrFilePath != "" {
		_, err = os.Stat(csrFilePath)
		if err != nil {
			log.Debugf("failed loading csr %s, continuing anyways", csrFilePath)
		} else {
			cert.Csr, err = clabutils.ReadFileContent(csrFilePath)
			if err != nil {
				return nil, err
			}
		}
	}

	return cert, nil
}

// Write writes the cert, key and csr to disk.
func (c *Certificate) Write(certPath, keyPath, csrPath string) error {
	log.Debugf("writing cert file to %s", certPath)

	err := clabutils.CreateFile(certPath, string(c.Cert))
	if err != nil {
		return err
	}

	log.Debugf("writing key file to %s", keyPath)
	err = clabutils.CreateFile(keyPath, string(c.Key))
	if err != nil {
		return err
	}

	// save csr if its length is >0 and path is not empty
	if len(c.Csr) != 0 && csrPath != "" {
		log.Debugf("writing csr file to %s", csrPath)

		err = clabutils.CreateFile(csrPath, string(c.Csr))
		if err != nil {
			return err
		}
	}

	return nil
}

type CaPaths interface {
	NodeCertAbsFilename(identifier string) string
	NodeCertKeyAbsFilename(identifier string) string
	NodeCertCSRAbsFilename(identifier string) string
	NodeTLSDir(string) string
	CaCertAbsFilename() string
	CaKeyAbsFilename() string
	CaCSRAbsFilename() string
}

package cert

import (
	"path"

	"github.com/srl-labs/containerlab/utils"
)

const (
	rootCaFilenamePrefix = "root-ca"
	certPostfix          = ".pem"
	keyPostfix           = "-key.pem"
	cSRPostfix           = ".csr"
)

// CertStorage defined the interface used to manage certificate storage
type CertStorage interface {
	LoadCaCert() (*Certificate, error)
	LoadNodeCert(nodeName string) (*Certificate, error)
	StoreCaCert(cert *Certificate) error
	StoreNodeCert(nodeName string, cert *Certificate) error
}

// CertStorageLocalDisk is a CertificateStorage implementation, that stores certificates in the given folder
type CertStorageLocalDisk struct {
	baseFolder string
}

// NewLocalDiskCertStorage inits a new NewLocalDiskCertStorage and returns a pointer to it
func NewLocalDiskCertStorage(baseFolder string) *CertStorageLocalDisk {
	return &CertStorageLocalDisk{
		baseFolder: baseFolder,
	}
}

// LoadCaCert loads and returns the CA certificat from disk or the error that occured while trying to read it
func (c *CertStorageLocalDisk) LoadCaCert() (*Certificate, error) {
	return c.LoadNodeCert(rootCaFilenamePrefix)
}

// LoadNodeCert loads and returns the certificat from disk matching the provided identifier or the error that occured while trying to read it
func (c *CertStorageLocalDisk) LoadNodeCert(nodeName string) (*Certificate, error) {
	certFilename := c.getCertAbsFilename(nodeName)
	keyFilename := c.getKeyAbsFilename(nodeName)
	csrFilename := c.getCSRAbsFilename(nodeName)
	return LoadCertificateFromDisk(certFilename, keyFilename, csrFilename)
}

// StoreCaCert stores the given CA certificate in a file in the baseFolder
func (c *CertStorageLocalDisk) StoreCaCert(cert *Certificate) error {
	return c.StoreNodeCert(rootCaFilenamePrefix, cert)
}

// StoreNodeCert stores the given certificate in a file in the baseFolder
func (c *CertStorageLocalDisk) StoreNodeCert(nodeName string, cert *Certificate) error {
	// create a folder for the node if it does not exist
	utils.CreateDirectory(path.Join(c.baseFolder, nodeName), 0777)

	// write cert files
	return cert.Write(c.getCertAbsFilename(nodeName), c.getKeyAbsFilename(nodeName), c.getCSRAbsFilename(nodeName))
}

// GetCertKeyAbsFilename returns the path to a key file for the given identifier
func (c *CertStorageLocalDisk) getKeyAbsFilename(identifier string) string {
	return path.Join(c.baseFolder, identifier, identifier+keyPostfix)
}

// GetCertKeyAbsFilename returns the path to a cert file for the given identifier
func (c *CertStorageLocalDisk) getCertAbsFilename(identifier string) string {
	return path.Join(c.baseFolder, identifier, identifier+certPostfix)
}

// GetCertKeyAbsFilename returns the path to a csr file for the given identifier
func (c *CertStorageLocalDisk) getCSRAbsFilename(identifier string) string {
	return path.Join(c.baseFolder, identifier, identifier+cSRPostfix)
}

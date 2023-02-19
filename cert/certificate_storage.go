package cert

import (
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

// CertStorage defines the interface used to manage certificate storage.
type CertStorage interface {
	LoadCaCert() (*Certificate, error)
	LoadNodeCert(nodeName string) (*Certificate, error)
	StoreCaCert(cert *Certificate) error
	StoreNodeCert(nodeName string, cert *Certificate) error
}

// LocalDirCertStorage is a certificate storage, that stores certificates in a local directory.
type LocalDirCertStorage struct {
	paths types.CaPaths
}

// NewLocalDirCertStorage inits a new LocalDirCertStorage.
func NewLocalDirCertStorage(paths types.CaPaths) *LocalDirCertStorage {
	return &LocalDirCertStorage{
		paths: paths,
	}
}

// LoadCaCert loads and returns the CA certificat from disk or the error that occured while trying to read it
func (c *LocalDirCertStorage) LoadCaCert() (*Certificate, error) {
	return c.LoadNodeCert(c.paths.RootCaIdentifier())
}

// LoadNodeCert loads and returns the certificat from disk matching the provided identifier or the error that occured while trying to read it
func (c *LocalDirCertStorage) LoadNodeCert(nodeName string) (*Certificate, error) {
	certFilename := c.paths.NodeCertAbsFilename(nodeName)
	keyFilename := c.paths.NodeCertKeyAbsFilename(nodeName)
	csrFilename := c.paths.NodeCertCSRAbsFilename(nodeName)
	return NewCertificateFromFile(certFilename, keyFilename, csrFilename)
}

// StoreCaCert stores the given CA certificate in a file in the baseFolder
func (c *LocalDirCertStorage) StoreCaCert(cert *Certificate) error {
	return c.StoreNodeCert(c.paths.RootCaIdentifier(), cert)
}

// StoreNodeCert stores the given certificate in a file in the baseFolder
func (c *LocalDirCertStorage) StoreNodeCert(nodeName string, cert *Certificate) error {
	// create a folder for the node if it does not exist
	utils.CreateDirectory(c.paths.CANodeDir(nodeName), 0777)

	// write cert files
	return cert.Write(c.paths.NodeCertAbsFilename(nodeName), c.paths.NodeCertKeyAbsFilename(nodeName), c.paths.NodeCertCSRAbsFilename(nodeName))
}

package cert

import (
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
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
	paths types.CaPaths
}

// NewLocalDiskCertStorage inits a new NewLocalDiskCertStorage and returns a pointer to it
func NewLocalDiskCertStorage(paths types.CaPaths) *CertStorageLocalDisk {
	return &CertStorageLocalDisk{
		paths: paths,
	}
}

// LoadCaCert loads and returns the CA certificat from disk or the error that occured while trying to read it
func (c *CertStorageLocalDisk) LoadCaCert() (*Certificate, error) {
	return c.LoadNodeCert(c.paths.RootCaIdentifier())
}

// LoadNodeCert loads and returns the certificat from disk matching the provided identifier or the error that occured while trying to read it
func (c *CertStorageLocalDisk) LoadNodeCert(nodeName string) (*Certificate, error) {
	certFilename := c.paths.NodeCertAbsFilename(nodeName)
	keyFilename := c.paths.NodeCertKeyAbsFilename(nodeName)
	csrFilename := c.paths.NodeCertCSRAbsFilename(nodeName)
	return LoadCertificateFromDisk(certFilename, keyFilename, csrFilename)
}

// StoreCaCert stores the given CA certificate in a file in the baseFolder
func (c *CertStorageLocalDisk) StoreCaCert(cert *Certificate) error {
	return c.StoreNodeCert(c.paths.RootCaIdentifier(), cert)
}

// StoreNodeCert stores the given certificate in a file in the baseFolder
func (c *CertStorageLocalDisk) StoreNodeCert(nodeName string, cert *Certificate) error {
	// create a folder for the node if it does not exist
	utils.CreateDirectory(c.paths.CANodeDir(nodeName), 0777)

	// write cert files
	return cert.Write(c.paths.NodeCertAbsFilename(nodeName), c.paths.NodeCertKeyAbsFilename(nodeName), c.paths.NodeCertCSRAbsFilename(nodeName))
}

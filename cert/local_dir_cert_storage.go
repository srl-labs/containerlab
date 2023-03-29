package cert

import (
	"path/filepath"

	"github.com/srl-labs/containerlab/utils"
)

// LocalDirCertStorage is a certificate storage, that stores certificates in a local directory.
type LocalDirCertStorage struct {
	paths CaPaths
}

// NewLocalDirCertStorage inits a new LocalDirCertStorage.
func NewLocalDirCertStorage(paths CaPaths) *LocalDirCertStorage {
	return &LocalDirCertStorage{
		paths: paths,
	}
}

// LoadCaCert loads the CA certificate from disk.
func (c *LocalDirCertStorage) LoadCaCert() (*Certificate, error) {
	return NewCertificateFromFile(c.paths.CaCertFile(), c.paths.CaKeyFile(), "")
}

// LoadNodeCert loads the node certificate from disk.
// Used to load CA certificate as well, as CA certificate can be seen as node named "ca".
func (c *LocalDirCertStorage) LoadNodeCert(nodeName string) (*Certificate, error) {
	certFilename := c.paths.NodeCertAbsFilename(nodeName)
	keyFilename := c.paths.NodeCertKeyAbsFilename(nodeName)
	csrFilename := c.paths.NodeCertCSRAbsFilename(nodeName)
	return NewCertificateFromFile(certFilename, keyFilename, csrFilename)
}

// StoreCaCert stores the given CA certificate in a file in the baseFolder.
func (c *LocalDirCertStorage) StoreCaCert(cert *Certificate) error {
	// create a folder for the node if it does not exist
	for _, d := range []string{c.paths.CaCertFile(), c.paths.CaKeyFile(), c.paths.CaCSRFile()} {
		// make sure the directory exists (probably all the same dir, but who knows)
		directory := filepath.Dir(d)
		utils.CreateDirectory(directory, 0777)
	}

	// write cert files
	return cert.Write(c.paths.CaCertFile(),
		c.paths.CaKeyFile(), c.paths.CaCSRFile())
}

// StoreNodeCert stores the given certificate in a file in the baseFolder.
func (c *LocalDirCertStorage) StoreNodeCert(nodeName string, cert *Certificate) error {
	// create a folder for the node if it does not exist
	utils.CreateDirectory(c.paths.NodeTLSDir(nodeName), 0777)

	// write cert files
	return cert.Write(c.paths.NodeCertAbsFilename(nodeName),
		c.paths.NodeCertKeyAbsFilename(nodeName), c.paths.NodeCertCSRAbsFilename(nodeName))
}

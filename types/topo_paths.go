package types

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ansibleInventoryFileName  = "ansible-inventory.yml"
	topologyExportDatFileName = "topology-data.json"
	authzKeysFileName         = "authorized_keys"
	caFolder                  = "ca"
	rootCaFolder              = "root"
	graph                     = "graph"
	labDirPrefix              = "clab-"
	backupFileSuffix          = ".bak"
	backupFilePrefix          = "."
)

// TopoPaths creates all the required absolute paths and filenames for a topology.
// generally all these paths are deduced from two main paths. The topology file path and the lab dir path.
type TopoPaths struct {
	topoFile string
	labDir   string
}

// NewTopoPaths constructs a new TopoPaths instance.
func NewTopoPaths(topologyFile string) (*TopoPaths, error) {
	t := &TopoPaths{}
	err := t.SetTopologyFilePath(topologyFile)
	if err != nil {
		return nil, err
	}

	return t, err
}

// SetTopologyFilePath sets the topology file path.
func (t *TopoPaths) SetTopologyFilePath(topologyFile string) error {
	absTopoFile, err := filepath.Abs(topologyFile)
	if err != nil {
		return err
	}

	// make sure topo file exists
	_, err = os.Stat(absTopoFile)
	if err != nil {
		return err
	}

	t.topoFile = absTopoFile

	return nil
}

// SetLabDir sets the labDir.
func (t *TopoPaths) SetLabDir(topologyName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	ldPath := path.Join(cwd, labDirPrefix+topologyName)

	t.labDir = ldPath

	return nil
}

// CABaseDir returns the root of the CA directory structure.
func (t *TopoPaths) CABaseDir() string {
	return path.Join(t.labDir, caFolder)
}

// CARootCertDir returns the directory that contains the root CA certificat and key.
func (t *TopoPaths) CARootCertDir() string {
	return path.Join(t.CABaseDir(), rootCaFolder)
}

// CANodeDir returns the directory that contains the certificat data for the given node.
func (t *TopoPaths) CANodeDir(nodename string) string {
	return path.Join(t.CABaseDir(), nodename)
}

// AuthorizedKeysFilename returns the path for the generated AuthorizedKeysFile.
func (t *TopoPaths) AuthorizedKeysFilename() string {
	return path.Join(t.labDir, authzKeysFileName)
}

// GraphDir returns the directory that takes the graphs.
func (t *TopoPaths) GraphDir() string {
	return path.Join(t.labDir, graph)
}

// GraphFilename returns the filename for a given graph file with the provided fileending.
func (t *TopoPaths) GraphFilename(fileEnding string) string {
	// if a fileending is provided and does not start with . add the .
	if len(fileEnding) > 0 && !strings.HasPrefix(fileEnding, ".") {
		fileEnding = "." + fileEnding
	}
	return path.Join(t.GraphDir(), t.TopologyFilenameWithoutExt()+fileEnding)
}

// NodeDir returns the directory in the labDir for the provided node.
func (t *TopoPaths) NodeDir(nodeName string) string {
	return path.Join(t.labDir, nodeName)
}

// TopoExportFile returns the path for the topology-export file.
func (t *TopoPaths) TopoExportFile() string {
	return path.Join(t.labDir, topologyExportDatFileName)
}

// AnsibleInventoryFileAbsPath returns the absolute path to the ansible-inventory file.
func (t *TopoPaths) AnsibleInventoryFileAbsPath() string {
	return path.Join(t.labDir, ansibleInventoryFileName)
}

// TopologyFilenameAbsPath returns the absolute path to the topology file.
func (t *TopoPaths) TopologyFilenameAbsPath() string {
	return t.topoFile
}

// TopologyFilenameBase returns the full filename of the topology file
// without any additional paths.
func (t *TopoPaths) TopologyFilenameBase() string {
	return filepath.Base(t.topoFile)
}

// TopologyFilenameWithoutExt returns the topology file name without the file extension.
func (t *TopoPaths) TopologyFilenameWithoutExt() string {
	baseName := t.TopologyFilenameBase()
	r := regexp.MustCompile(`[\.-]clab\.`)
	loc := r.FindStringIndex(baseName)
	if len(loc) > 0 {
		return baseName[0:loc[0]]
	}
	return strings.TrimSuffix(baseName, path.Ext(baseName))
}

func (t *TopoPaths) TopologyFileIsSet() bool {
	if t == nil {
		return false
	}
	return t.topoFile != ""
}

// TopologyBakFileAbsPath returns the backup topology file name.
func (t *TopoPaths) TopologyBakFileAbsPath() string {
	return path.Join(t.TopologyFileDir(), backupFilePrefix+t.TopologyFilenameBase()+backupFileSuffix)
}

// TopologyFileDir returns the abs path to the topology file directory.
func (t *TopoPaths) TopologyFileDir() string {
	return filepath.Dir(t.topoFile)
}

// TopologyLabDir returns the lab directory.
func (t *TopoPaths) TopologyLabDir() string {
	return t.labDir
}

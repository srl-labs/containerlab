package types

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ansibleInventory   = "ansible-inventory.yml"
	topologyExportData = "topology-data.json"
	authzKeys          = "authorized_keys"
	caFolder           = "ca"
	rootCaFolder       = "root"
	graph              = "graph"
	workDirPrefix      = "clab-"
	backupFileSuffix   = ".bak"
	backupFilePrefix   = "."
)

// TopoPaths creates all the required absolute paths and filenames for a topology.
// generally all these paths are deduced from two main paths. The topology file path and the work dir path.
type TopoPaths struct {
	topologyConfigFile string
	topologyWorkDir    string
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

func (t *TopoPaths) SetTopologyName(topologyName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	wd_path := path.Join(cwd, workDirPrefix+topologyName)
	return t.SetWorkDir(wd_path)
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
	t.topologyConfigFile = absTopoFile
	return nil
}

// SetWorkDir explicitly sets the WorkDir. Use UpdateWorkDir to deduce workdir from CWD/"clab-"+Filename of the topology file.
func (t *TopoPaths) SetWorkDir(workDir string) error {
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return err
	}
	t.topologyWorkDir = absWorkDir
	return nil
}

// GetCABaseDir returns the root of the CA directory structure.
func (t *TopoPaths) GetCABaseDir() string {
	return path.Join(t.topologyWorkDir, caFolder)
}

// GetCARootCertDir returns the directory that contains the root CA certificat and key.
func (t *TopoPaths) GetCARootCertDir() string {
	return path.Join(t.GetCABaseDir(), rootCaFolder)
}

// GetCANodeDir returns the directory that contains the certificat data for the given node.
func (t *TopoPaths) GetCANodeDir(nodename string) string {
	return path.Join(t.GetCABaseDir(), nodename)
}

// GetAuthorizedKeysFilename returns the path for the generated AuthorizedKeysFile.
func (t *TopoPaths) GetAuthorizedKeysFilename() string {
	return path.Join(t.topologyWorkDir, authzKeys)
}

// GetGraphDir returns the directory that takes the graphs.
func (t *TopoPaths) GetGraphDir() string {
	return path.Join(t.topologyWorkDir, graph)
}

// GetGraphFilename returns the filename for a given graph file with the provided fileending.
func (t *TopoPaths) GetGraphFilename(fileEnding string) string {
	// if a fileending is provided and does not start with . add the .
	if len(fileEnding) > 0 && !strings.HasPrefix(fileEnding, ".") {
		fileEnding = "." + fileEnding
	}
	return path.Join(t.GetGraphDir(), t.GetTopologyFilename()+fileEnding)
}

// GetNodeDir returns the working dir for the provided node.
func (t *TopoPaths) GetNodeDir(nodeName string) string {
	return path.Join(t.topologyWorkDir, nodeName)
}

// GetTopoExportFile returns the path for the topology-export file.
func (t *TopoPaths) GetTopoExportFile() string {
	return path.Join(t.topologyWorkDir, topologyExportData)
}

// GetAnsibleInventoryFileAbs returns the path to the ansible-inventory.
func (t *TopoPaths) GetAnsibleInventoryFileAbs() string {
	return path.Join(t.topologyWorkDir, ansibleInventory)
}

// GetTopologyFilenameAbs returns the absolute path to the topology file.
func (t *TopoPaths) GetTopologyFilenameAbs() string {
	return t.topologyConfigFile
}

// GetTopologyFilenameFull returns the full filename of the topology file
// without any additional paths.
func (t *TopoPaths) GetTopologyFilenameFull() string {
	return filepath.Base(t.topologyConfigFile)
}

// GetTopologyFilename returns the topology file name, truncated by the file extension.
func (t *TopoPaths) GetTopologyFilename() string {
	baseName := t.GetTopologyFilenameFull()
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
	return t.topologyConfigFile != ""
}

// GetTopologyBakFileAbs returns the backup topology file name.
func (t *TopoPaths) GetTopologyBakFileAbs() string {
	return path.Join(t.GetTopologyFileDir(), backupFilePrefix+t.GetTopologyFilenameFull()+backupFileSuffix)
}

// GetTopologyFileDir returns the abs path to the topology file directory.
func (t *TopoPaths) GetTopologyFileDir() string {
	return filepath.Dir(t.topologyConfigFile)
}

// GetTopologyWorkDir returns the workdir.
func (t *TopoPaths) GetTopologyWorkDir() string {
	return t.topologyWorkDir
}

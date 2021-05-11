package clab

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"gopkg.in/yaml.v2"
)

// TopoFile type is a struct which defines parameters of the topology file
type TopoFile struct {
	path     string // topo file path
	fullName string // file name with extension
	name     string // file name without extension
}

// GetTopology parses the topology file into c.Conf structure
// as well as populates the TopoFile structure with the topology file related information
func (c *CLab) GetTopology(topo string) error {
	yamlFile, err := ioutil.ReadFile(topo)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Topology file contents:\n%s\n", yamlFile))

	err = yaml.UnmarshalStrict(yamlFile, c.Config)
	if err != nil {
		return err
	}

	path, _ := filepath.Abs(topo)
	if err != nil {
		return err
	}

	s := strings.Split(topo, "/")
	file := s[len(s)-1]
	filename := strings.Split(file, ".")
	c.TopoFile = &TopoFile{
		path:     path,
		fullName: file,
		name:     filename[0],
	}
	return nil
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, copy the file contents from src to dst.
func copyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	return copyFileContents(src, dst)
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func createFile(file, content string) {
	var f *os.File
	f, err := os.Create(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		panic(err)
	}
}

// CreateNodeDirStructure create the directory structure and files for the lab nodes
func (c *CLab) CreateNodeDirStructure(node *types.Node) (err error) {
	c.m.RLock()
	defer c.m.RUnlock()

	// create node directory in the lab directory
	// skip creation of node directory for linux/bridge kinds
	// since they don't keep any state normally
	if node.Kind != "linux" && node.Kind != "bridge" {
		utils.CreateDirectory(node.LabDir, 0777)
	}

	switch node.Kind {
	case "srl":
		if err := c.createSRLFiles(node); err != nil {
			return err
		}
	case "ceos":
		if err := c.createCEOSFiles(node); err != nil {
			return err
		}
	case "crpd":
		if err := c.createCRPDFiles(node); err != nil {
			return err
		}
	case "vr-sros":
		if err := c.createVrSROSFiles(node); err != nil {
			return err
		}
	}
	return nil
}

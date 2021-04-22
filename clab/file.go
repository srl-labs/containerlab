package clab

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
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

func fileExists(filename string) bool {
	f, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !f.IsDir()
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

// CreateDirectory creates a directory by a path with a mode/permission specified by perm.
// If directory exists, the function does not do anything.
func CreateDirectory(path string, perm os.FileMode) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, perm)
	}
}

// CreateNodeDirStructure create the directory structure and files for the lab nodes
func (c *CLab) CreateNodeDirStructure(node *Node) (err error) {
	c.m.RLock()
	defer c.m.RUnlock()

	// create node directory in the lab directory
	// skip creation of node directory for linux/bridge kinds
	// since they don't keep any state normally
	if node.Kind != "linux" && node.Kind != "bridge" {
		CreateDirectory(node.LabDir, 0777)
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

// GenerateConfig generates configuration for the nodes
func (node *Node) generateConfig(dst string) error {
	if fileExists(dst) && (node.Config == defaultConfigTemplates[node.Kind]) && node.Kind != "ceos" {
		log.Debugf("config file '%s' for node '%s' already exists and will not be generated", dst, node.ShortName)
		return nil
	}
	log.Debugf("generating config for node %s from file %s", node.ShortName, node.Config)
	tpl, err := template.New(filepath.Base(node.Config)).ParseFiles(node.Config)
	if err != nil {
		return err
	}
	dstBytes := new(bytes.Buffer)
	err = tpl.Execute(dstBytes, node)
	if err != nil {
		return err
	}
	log.Debugf("node '%s' generated config: %s", node.ShortName, dstBytes.String())
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(dstBytes.Bytes())
	return err
}

func readFileContent(file string) ([]byte, error) {
	// check file exists
	if !fileExists(file) {
		return nil, fmt.Errorf("file %s does not exist", file)
	}

	// read and return file content
	b, err := ioutil.ReadFile(file)
	return b, err
}

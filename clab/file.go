package clab

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// File type is a struct which define parameter of the topology file
type File struct {
	file      string
	name      string
	shortname string
}

// GetTopology gets the lab topology information
func (c *cLab) GetTopology(topo *string) error {
	log.Info("Getting topology information ...")
	log.Debug("Topofile ", *topo)

	yamlFile, err := ioutil.ReadFile(*topo)
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("File contents:\n%s\n", yamlFile))

	err = yaml.Unmarshal(yamlFile, c.Conf)
	if err != nil {
		return err
	}

	s := strings.Split(*topo, "/")
	file := s[len(s)-1]
	filename := strings.Split(file, ".")
	sf := strings.Split(filename[0], "-")
	shortname := ""
	for _, f := range sf {
		shortname += f
	}
	log.Debug(s, file, filename, shortname)
	c.FileInfo = &File{
		file:      file,
		name:      filename[0],
		shortname: shortname,
	}
	log.Debug("File : ", c.FileInfo)

	return nil

}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	log.Debug(info)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, copy the file contents from src to dst.
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
		os.Mkdir(path, perm)
	}
}

// CreateNodeDirStructure create the directory structure and files for the clab
func (c *cLab) CreateNodeDirStructure(node *Node) (err error) {
	switch node.Kind {
	case "srl":
		log.Infof("Create directory structure for SRL container: %s", node.ShortName)
		var src string
		var dst string
		// copy license file to node specific directory in lab
		src = node.License
		dst = path.Join(c.Dir.Lab, "license.key")
		if err = copyFile(src, dst); err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
		}
		log.Debugf("CopyFile src %s -> dst %s succeeded", src, dst)

		// create dut directory in lab
		CreateDirectory(node.LabDir, 0777)
		// generate SRL topology file
		err = generateSRLTopologyFile(node.Topology, node.LabDir, node.Index)
		if err != nil {
			log.Fatalln(err)
		}

		// generate a config file if the destination does not exist
		CreateDirectory(path.Join(node.LabDir, "config"), 0777)
		dst = path.Join(node.LabDir, "config", "config.json")
		if !fileExists(dst) {
			err = node.generateConfig(dst)
			if err != nil {
				log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
			}
		} else {
			log.Debugf("Config File Exists for node %s", node.ShortName)
		}
		node.Config = dst

		// copy env config to node specific directory in lab
		src = "/etc/containerlab/templates/srl/srl_env.conf"
		dst = node.LabDir + "/" + "srlinux.conf"
		err = copyFile(src, dst)
		if err != nil {
			return fmt.Errorf("CopyFile src %s -> dst %s failed %v", src, dst, err)
		}
		log.Debug(fmt.Sprintf("CopyFile src %s -> dst %s succeeded\n", src, dst))
		node.EnvConf = dst

	case "alpine":
	case "linux":
	case "ceos":
	case "bridge":
	default:
	}

	return nil
}

func (c *cLab) CreateHostsFile() {
	var v4Hosts []string
	var v6Hosts []string
	for dutName, node := range c.Nodes {
		if node.Kind != "bridge" {
			log.Infof("Mgmt IP addresses of container: %s, ContainerName: %s, IPv4: %s, IPv6: %s, MAC: %s", dutName, node.LongName, node.MgmtIPv4, node.MgmtIPv6, node.MgmtMac)
			v4Hosts = append(v4Hosts, fmt.Sprintf("%s \t\t\t %s\n", node.MgmtIPv4, node.LongName))
			v6Hosts = append(v6Hosts, fmt.Sprintf("%s \t\t %s\n", node.MgmtIPv6, node.LongName))
		}
	}

	hosts := append(v4Hosts, v6Hosts...)
	createFile(path.Join(c.Dir.Lab, "hosts"), strings.Join(hosts, ""))
	log.Infof("Generated hosts filename: %s", path.Join(c.Dir.Lab, "hosts"))
}

// GenerateConfig generates configuration for the duts
func (node *Node) generateConfig(dst string) error {
	tpl, err := template.New("srlconfig.tpl").ParseFiles("/etc/containerlab/templates/srl/srlconfig.tpl")
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

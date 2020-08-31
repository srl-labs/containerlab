package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
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

// FileInfo global variable stores information on the filename
var FileInfo *File

func (c *cLab) getTopology(topo *string) error {
	log.Debug("Topofile ", *topo)

	yamlFile, err := ioutil.ReadFile(*topo)
	if err != nil {
		log.Error(err)
	}
	log.Debug(fmt.Sprintf("File contents:\n%s\n", yamlFile))

	//var t *conf
	err = yaml.Unmarshal(yamlFile, c.Conf)
	if err != nil {
		log.Error(err)
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

func linkFile(src, dst string) (err error) {
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
			return err
		}
	}
	err = os.Link(src, dst)
	if err != nil {
		return err
	}
	return nil
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
	err = copyFileContents(src, dst)
	return nil
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

func createDirectory(path string, perm os.FileMode) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, perm)
	}
}

func (c *cLab) createNodeDirStructure(node *Node, dut string) (err error) {
	// create lab directory
	path := c.Conf.ConfigPath + "/" + "lab" + "-" + c.Conf.Prefix

	switch node.OS {
	case "srl":
		var src string
		var dst string
		// copy license file to node specific directory in lab
		src = node.License
		dst = path + "/" + "license.key"
		if err = copyFile(src, dst); err != nil {
			log.Error(fmt.Sprintf("CopyFile src %s -> dat %s failed %q\n", src, dst, err))
			return err
		}
		log.Debug(fmt.Sprintf("CopyFile src %s -> dat %s succeeded\n", src, dst))

		// create dut directory in lab
		path += "/" + dut
		createDirectory(path, 0777)
		node.Path = path

		// copy topology to node specific directory in lab
		src = node.Topology
		dst = path + "/" + "topology.yml"
		tpl, err := template.ParseFiles(src)
		if err != nil {
			log.Fatalln(err)
		}
		type Mac struct {
			MAC string
		}
		x := strconv.FormatInt(int64(node.Index), 16)
		d2 := fmt.Sprintf("%02s", x)
		m := "00:01:" + strings.ToUpper(d2) + ":00:00:00"
		mac := Mac{
			MAC: m,
		}
		log.Debug(mac, dst)
		f, err := os.Create(dst)
		if err != nil {
			log.Error("create file: ", err)
			return err
		}
		defer f.Close()

		if err = tpl.Execute(f, mac); err != nil {
			panic(err)
		}
		log.Debug(fmt.Sprintf("CopyFile GoTemplate src %s -> dat %s succeeded\n", src, dst))

		// copy config file to node specific directory in lab

		createDirectory(path+"/"+"config", 0777)
		src = node.Config
		dst = path + "/" + "config" + "/" + "config.json"
		if !fileExists(dst) {
			err = copyFile(src, dst)
			if err != nil {
				log.Error(fmt.Sprintf("CopyFile src %s -> dat %s failed %q\n", src, dst, err))
				return err
			}
			log.Debug(fmt.Sprintf("CopyFile src %s -> dat %s succeeded\n", src, dst))
		} else {
			log.Debug("Config File Exists")
		}
		node.Config = dst

		// copy env config to node specific directory in lab

		src = "srl_config/srl_env.conf"
		dst = path + "/" + "srlinux.conf"
		err = copyFile(src, dst)
		if err != nil {
			log.Error(fmt.Sprintf("CopyFile src %s -> dat %s failed %q\n", src, dst, err))
			return err
		}
		log.Debug(fmt.Sprintf("CopyFile src %s -> dat %s succeeded\n", src, dst))
		node.EnvConf = dst

	case "alpine":
	}

	return nil
}

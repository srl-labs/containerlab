package clab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"text/template"

	log "github.com/sirupsen/logrus"
)

func cfssljson(b []byte, file string, node *Node) {
	var input = map[string]interface{}{}
	var err error
	var cert string
	var key string
	var csr string

	//log.Debugf("cfssl output:\n%s", string(b))
	err = json.Unmarshal(b, &input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input: %v\n", err)
		os.Exit(1)
	}
	if contents, ok := input["cert"]; ok {
		cert = contents.(string)
		if node != nil {
			node.TLSCert = cert
		}
	}
	createFile(file+".pem", cert)

	if contents, ok := input["key"]; ok {
		key = contents.(string)
		if node != nil {
			node.TLSKey = key // TODO: figure out how to transform key bytes before storing
		}
	}
	createFile(file+"-key.pem", key)

	if contents, ok := input["csr"]; ok {
		csr = contents.(string)
	}
	createFile(file+".csr", csr)
	if node != nil {
		log.Debugf("node: %+v", node)
	}
}

// CreateRootCA creates a root CA
func (c *cLab) CreateRootCA() (err error) {
	//create root CA diretcory
	CreateDirectory(c.Dir.LabCA, 0755)

	//create root CA root diretcory
	CreateDirectory(c.Dir.LabCARoot, 0755)

	var src string
	var dst string

	// copy topology to node specific directory in lab
	src = "/etc/containerlab/templates/ca/csr-root-ca.json"
	dst = c.Dir.LabCARoot + "/" + "csr-root-ca.json"
	tpl, err := template.ParseFiles(src)
	if err != nil {
		log.Fatalln(err)
	}
	type Prefix struct {
		Prefix string
	}
	prefix := Prefix{
		Prefix: "containerlab" + "-" + c.Conf.Prefix,
	}
	f, err := os.Create(dst)
	if err != nil {
		log.Error("create file: ", err)
		return err
	}
	defer f.Close()

	if err = tpl.Execute(f, prefix); err != nil {
		panic(err)
	}
	log.Debug(fmt.Sprintf("CopyFile GoTemplate src %s -> dat %s succeeded\n", src, dst))

	cmd := exec.Command("cfssl", "gencert", "-initca", dst)
	o, err := cmd.Output()
	if err != nil {
		log.Errorf("cmd.Run() failed with %s", err)
	}
	if debug {
		jsCert := new(bytes.Buffer)
		json.Indent(jsCert, o, "", "  ")
		log.Debugf("'cfssl gencert -initca' output:\n%s", jsCert.String())
	}

	cfssljson(o, c.Dir.LabCARoot+"/"+"root-ca", nil)

	return nil
}

// CreateCERT create a certificate
func (c *cLab) CreateCERT(shortdutName string) (err error) {
	node, ok := c.Nodes[shortdutName]
	if !ok {
		return fmt.Errorf("unknown dut name: %s", shortdutName)
	}
	//create dut cert diretcory
	CreateDirectory(c.Nodes[shortdutName].CertDir, 0755)

	var src string
	var dst string

	// copy topology to node specific directory in lab
	src = "/etc/containerlab/templates/ca/csr.json"
	dst = path.Join(node.CertDir, "csr"+"-"+shortdutName+".json")
	tpl, err := template.ParseFiles(src)
	if err != nil {
		log.Fatalln(err)
	}
	type CERT struct {
		Name   string
		Prefix string
	}
	cert := CERT{
		Name:   shortdutName,
		Prefix: c.Conf.Prefix,
	}
	f, err := os.Create(dst)
	if err != nil {
		log.Error("create file: ", err)
		return err
	}
	defer f.Close()

	if err = tpl.Execute(f, cert); err != nil {
		panic(err)
	}
	log.Debug(fmt.Sprintf("CopyFile GoTemplate src %s -> dat %s succeeded\n", src, dst))

	var cmd *exec.Cmd
	rootCert := path.Join(c.Dir.LabCARoot, "root-ca.pem")
	rootKey := path.Join(c.Dir.LabCARoot, "root-ca-key.pem")
	cmd = exec.Command("cfssl", "gencert", "-ca", rootCert, "-ca-key", rootKey, dst)
	o, err := cmd.Output()
	if err != nil {
		log.Errorf("'cfssl gencert -ca rootCert -caKey rootKey' failed with: %v", err)
	}
	if debug {
		jsCert := new(bytes.Buffer)
		json.Indent(jsCert, o, "", "  ")
		log.Debugf("'cfssl gencert -ca rootCert -caKey rootKey' output:\n%s", jsCert.String())
	}

	cfssljson(o, path.Join(node.CertDir, shortdutName), node)
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
)

func parsecfsslInput(i *[]byte) (out string) {
	in := strings.Split(string(*i), "{")
	for i, s := range in {
		if i != 0 {
			out += s
		}
	}
	return "{" + out
}

func cfssljson(i *[]byte, file string) {
	var input = map[string]interface{}{}
	var err error
	var cert string
	var key string
	var csr string

	err = json.Unmarshal([]byte(parsecfsslInput(i)), &input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input: %v\n", err)
		os.Exit(1)
	}
	if contents, ok := input["cert"]; ok {
		cert = contents.(string)
	} else if contents, ok = input["certificate"]; ok {
		cert = contents.(string)
	}
	createFile(file+".pem", cert)

	if contents, ok := input["key"]; ok {
		key = contents.(string)
	} else if contents, ok = input["private_key"]; ok {
		key = contents.(string)
	}
	createFile(file+"-key.pem", key)

	if contents, ok := input["csr"]; ok {
		csr = contents.(string)
	} else if contents, ok = input["certificate_request"]; ok {
		csr = contents.(string)
	}
	createFile(file+".csr", csr)

}

func (c *cLab) createRootCA() (err error) {
	//create root CA diretcory
	createDirectory(c.Dir.LabCA, 0755)

	//create root CA root diretcory
	createDirectory(c.Dir.LabCARoot, 0755)

	var src string
	var dst string

	// copy topology to node specific directory in lab
	src = "ca_config/templates/csr-root-ca.json"
	dst = c.Dir.LabCARoot + "/" + "csr-root-ca.json"
	tpl, err := template.ParseFiles(src)
	if err != nil {
		log.Fatalln(err)
	}
	type Prefix struct {
		Prefix string
	}
	prefix := Prefix{
		Prefix: "lab" + "-" + c.Conf.Prefix,
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

	var cmd *exec.Cmd
	cmd = exec.Command("/home/henderiw/work/bin/cfssl", "gencert", "-initca", dst)
	o, err := cmd.CombinedOutput()
	//fmt.Println(string(o))
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
	}

	cfssljson(&o, c.Dir.LabCARoot+"/"+"root-ca")

	return nil
}

func (c *cLab) createCERT(shortdutName string) (err error) {
	//create dut cert diretcory
	createDirectory(c.Nodes[shortdutName].CertDir, 0755)

	var src string
	var dst string

	// copy topology to node specific directory in lab
	src = "ca_config/templates/csr.json"
	dst = c.Nodes[shortdutName].CertDir + "/" + "csr" + "-" + shortdutName + ".json"
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
	rootCert := c.Dir.LabCARoot + "/" + "root-ca.pem"
	rootKey := c.Dir.LabCARoot + "/" + "root-ca-key.pem"
	cmd = exec.Command("/home/henderiw/work/bin/cfssl", "gencert", "-ca", rootCert, "-ca-key", rootKey, dst)
	o, err := cmd.CombinedOutput()
	//fmt.Println(string(o))
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
	}

	cfssljson(&o, c.Nodes[shortdutName].CertDir+"/"+shortdutName)

	return nil
}

package clab

import (
	"bytes"
	"encoding/json"
	"path"
	"text/template"

	"github.com/cloudflare/cfssl/api/generator"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/universal"
	log "github.com/sirupsen/logrus"
)

type certificates struct {
	Key  []byte
	Csr  []byte
	Cert []byte
}

// CertInput struct
type CertInput struct {
	Name     string
	LongName string
	Fqdn     string
	Prefix   string
}

// CaRootInput struct
type CaRootInput struct {
	Prefix string
	Names  map[string]string // Not used right now
}

// GenerateRootCa function
func (c *cLab) GenerateRootCa(csrRootJsonTpl *template.Template, input CaRootInput) (*certificates, error) {
	log.Info("Creating root CA")
	//create root CA diretcory
	CreateDirectory(c.Dir.LabCA, 0755)

	//create root CA root diretcory
	CreateDirectory(c.Dir.LabCARoot, 0755)
	var err error
	csrBuff := new(bytes.Buffer)
	err = csrRootJsonTpl.Execute(csrBuff, input)
	if err != nil {
		return nil, err
	}
	req := csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
	}
	err = json.Unmarshal(csrBuff.Bytes(), &req)
	if err != nil {
		return nil, err
	}
	//
	var key, csrPEM, cert []byte
	cert, csrPEM, key, err = initca.New(&req)
	if err != nil {
		return nil, err
	}
	certs := &certificates{
		Key:  key,
		Csr:  csrPEM,
		Cert: cert,
	}
	c.writeCertFiles(certs, path.Join(c.Dir.LabCARoot, "root-ca"))
	return certs, nil
}

func (c *cLab) GenerateCert(ca string, caKey string, csrJSONTpl *template.Template, node *Node) (*certificates, error) {
	c.m.RLock()
	defer c.m.RUnlock()
	input := CertInput{
		Name:     node.ShortName,
		LongName: node.LongName,
		Fqdn:     node.Fqdn,
		Prefix:   c.Conf.Prefix,
	}
	CreateDirectory(path.Join(c.Dir.LabCA, input.Name), 0755)
	var err error
	csrBuff := new(bytes.Buffer)
	err = csrJSONTpl.Execute(csrBuff, input)
	if err != nil {
		return nil, err
	}

	req := &csr.CertificateRequest{
		KeyRequest: csr.NewKeyRequest(),
	}
	err = json.Unmarshal(csrBuff.Bytes(), req)
	if err != nil {
		return nil, err
	}

	var key, csrBytes []byte
	gen := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err = gen.ProcessRequest(req)
	if err != nil {
		return nil, err
	}

	policy := &config.Signing{
		Profiles: map[string]*config.SigningProfile{},
		Default:  config.DefaultConfig(),
	}
	root := universal.Root{
		Config: map[string]string{
			"cert-file": ca,
			"key-file":  caKey,
		},
		ForceRemote: false,
	}
	s, err := universal.NewSigner(root, policy)
	if err != nil {
		return nil, err
	}

	var cert []byte
	signReq := signer.SignRequest{
		Request: string(csrBytes),
	}
	cert, err = s.Sign(signReq)
	if err != nil {
		return nil, err
	}
	if len(signReq.Hosts) == 0 && len(req.Hosts) == 0 {
		log.Warning(generator.CSRNoHostMessage)
	}
	certs := &certificates{
		Key:  key,
		Csr:  csrBytes,
		Cert: cert,
	}
	//
	c.writeCertFiles(certs, path.Join(c.Dir.LabCA, input.Name, input.Name))
	return certs, nil
}

func (c *cLab) writeCertFiles(certs *certificates, filesPrefix string) {
	createFile(filesPrefix+".pem", string(certs.Cert))
	createFile(filesPrefix+"-key.pem", string(certs.Key))
	createFile(filesPrefix+".csr", string(certs.Csr))
}

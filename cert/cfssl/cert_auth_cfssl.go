package cfssl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/cloudflare/cfssl/api/generator"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	log "github.com/sirupsen/logrus"
	cert "github.com/srl-labs/containerlab/cert"
)

// CA is a Certificate Authority.
type CA struct {
	rootCert  *cert.Certificate
	signer    signer.Signer
	certStore cert.CertStorage
}

// NewCA initializes a Certificate Authority.
func NewCA(certStorage cert.CertStorage, debug bool) *CA {
	// setup loglevel for cfssl
	cfssllog.Level = cfssllog.LevelError
	if debug {
		cfssllog.Level = cfssllog.LevelDebug
	}

	return &CA{
		rootCert:  nil,
		certStore: certStorage,
	}
}

// SetRootCertificate tries to load the root certificat if it fails returns an error
func (ca *CA) SetRootCertificate(caCert *cert.Certificate) error {
	ca.rootCert = caCert
	return ca.initCaStructs()
}

// initCaStructs initializes the CA internal structs. It is meant to be called after either generating a new CA cert or
func (ca *CA) initCaStructs() error {
	var err error

	// init signingConf
	signingConf := &config.Signing{
		Profiles: map[string]*config.SigningProfile{},
		Default:  config.DefaultConfig(),
	}

	// set signer
	ca.signer, err = NewSignerFromCertificate(ca.rootCert, signingConf)
	if err != nil {
		return err
	}

	return nil
}

// GenerateRootCert generates a new RootCA
func (ca *CA) GenerateRootCert(input *cert.CsrInputCa) (*cert.Certificate, error) {
	log.Debug("Creating root CA")
	var err error

	// parse template for the RootCertCSR
	tpl, err := template.New("ca-csr").Parse(rootCACSRTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Root CA CSR Template: %v", err)
	}

	certSignReq, err := templatetoCSR(tpl, input)
	if err != nil {
		return nil, err
	}

	var keyBytes, csrPEMBytes, certBytes []byte
	certBytes, csrPEMBytes, keyBytes, err = initca.New(certSignReq)
	if err != nil {
		return nil, err
	}
	ca.rootCert = &cert.Certificate{
		Key:  keyBytes,
		Csr:  csrPEMBytes,
		Cert: certBytes,
	}
	return ca.rootCert, ca.initCaStructs()
}

// templatetoCSR converts a given template with its input into a *csr.CertificateRequest
func templatetoCSR(csrJSONTpl *template.Template, input any) (*csr.CertificateRequest, error) {
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
	return req, nil
}

// GenerateNodeCert generates and signs a certificate passed as input
func (ca *CA) GenerateNodeCert(input *cert.CsrInputNode) (*cert.Certificate, error) {
	// parse the nodeCSRTemplate
	certTpl, err := template.New("node-cert").Parse(NodeCSRTemplate)
	if err != nil {
		log.Errorf("failed to parse Node CSR Template: %v", err)
	}

	// generate certrequest via tempalte and input
	certreq, err := templatetoCSR(certTpl, input)
	if err != nil {
		return nil, err
	}

	// generate a cert key
	var key, csrBytes []byte
	gen := &csr.Generator{Validator: genkey.Validator}
	csrBytes, key, err = gen.ProcessRequest(certreq)
	if err != nil {
		return nil, err
	}

	// init sign request
	signReq := signer.SignRequest{
		Request: string(csrBytes),
	}
	// sign cert
	var certBytes []byte
	certBytes, err = ca.signer.Sign(signReq)
	if err != nil {
		return nil, err
	}
	// perform checks
	if len(certreq.Hosts) == 0 && len(signReq.Hosts) == 0 {
		log.Warning(generator.CSRNoHostMessage)
	}

	// construct result
	result := &cert.Certificate{
		Key:  key,
		Csr:  csrBytes,
		Cert: certBytes,
	}
	return result, nil
}

// NewSignerFromCertificate will init a new signer from the internal *cert.Certificate type
func NewSignerFromCertificate(caCert *cert.Certificate, policy *config.Signing) (signer.Signer, error) {
	parsedCa, err := helpers.ParseCertificatePEM(caCert.Cert)
	if err != nil {
		return nil, err
	}

	strPassword := os.Getenv("CFSSL_CA_PK_PASSWORD")
	password := []byte(strPassword)
	if strPassword == "" {
		password = nil
	}

	priv, err := helpers.ParsePrivateKeyPEMWithPassword(caCert.Key, password)
	if err != nil {
		log.Debugf("Malformed private key %v", err)
		return nil, err
	}

	return local.NewSigner(priv, parsedCa, signer.DefaultSigAlgo(priv), policy)
}

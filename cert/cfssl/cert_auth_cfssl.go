package cfssl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/cert"
)

// CA is a Certificate Authority.
type CA struct {
	rootCert *cert.Certificate
	signer   signer.Signer
}

// NewCA initializes a Certificate Authority.
func NewCA(debug bool) *CA {
	// setup loglevel for cfssl
	cfssllog.Level = cfssllog.LevelError
	if debug {
		cfssllog.Level = cfssllog.LevelDebug
	}

	return &CA{
		rootCert: nil,
	}
}

// SetCACert sets the CA certificate with the provided certificate and initializes the signer.
func (ca *CA) SetCACert(caCert *cert.Certificate) error {
	ca.rootCert = caCert
	return ca.initSigner()
}

// initSigner inits the signer for the CA.
func (ca *CA) initSigner() error {
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

// GenerateCACert generates a new CA certificate and key based on the CSR input.
func (ca *CA) GenerateCACert(input *cert.CACSRInput) (*cert.Certificate, error) {
	log.Debug("Creating root CA certificate and key")
	var err error

	csr, err := csrFromInput(input)
	if err != nil {
		return nil, err
	}

	var keyBytes, csrPEMBytes, certBytes []byte

	certBytes, csrPEMBytes, keyBytes, err = initca.New(csr)
	if err != nil {
		return nil, err
	}

	ca.rootCert = &cert.Certificate{
		Key:  keyBytes,
		Csr:  csrPEMBytes,
		Cert: certBytes,
	}

	err = ca.initSigner()
	if err != nil {
		return nil, err
	}

	return ca.rootCert, nil
}

// csrFromInput creates a new *csr.CertificateRequest from the input.
// Based on the input type it will use the appropriate template.
func csrFromInput(input any) (*csr.CertificateRequest, error) {
	var err error
	var tpl *template.Template

	switch input.(type) {
	case *cert.CACSRInput:
		tpl, err = template.New("ca-csr").Parse(CACSRTemplate)
	case *cert.NodeCSRInput:
		tpl, err = template.New("node-csr").Parse(NodeCSRTemplate)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	csrBuff := new(bytes.Buffer)
	err = tpl.Execute(csrBuff, input)
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

// GenerateAndSignNodeCert generates certificate and signs an end-user (node) certificate based on the NodeCSR input.
func (ca *CA) GenerateAndSignNodeCert(input *cert.NodeCSRInput) (*cert.Certificate, error) {
	nodeCsr, err := csrFromInput(input)
	if err != nil {
		return nil, err
	}

	// process csr request
	var keyBytes, csrBytes []byte
	gen := &csr.Generator{Validator: genkey.Validator}
	csrBytes, keyBytes, err = gen.ProcessRequest(nodeCsr)
	if err != nil {
		return nil, err
	}

	// init sign request
	signReq := signer.SignRequest{
		Request: string(csrBytes),
	}

	// sign cert with CA
	var certBytes []byte
	certBytes, err = ca.signer.Sign(signReq)
	if err != nil {
		return nil, err
	}

	result := &cert.Certificate{
		Cert: certBytes,
		Key:  keyBytes,
		Csr:  csrBytes,
	}

	return result, nil
}

// NewSignerFromCertificate inits a new signer from the internal *cert.Certificate type.
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

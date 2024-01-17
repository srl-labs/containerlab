package cert

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// CA is a Certificate Authority.
type CA struct {
	key  crypto.PrivateKey
	cert *x509.Certificate
}

// NewCA initializes a Certificate Authority.
func NewCA() *CA {
	return &CA{}
}

// SetCACert sets the CA certificate with the provided certificate and key.
func (ca *CA) SetCACert(cert *Certificate) error {
	var err error

	// PEM to DER
	pbCert, _ := pem.Decode(cert.Cert)

	// parse the Certificate
	ca.cert, err = x509.ParseCertificate(pbCert.Bytes)
	if err != nil {
		return err
	}

	// Parse the PrivateKey
	ca.key, err = ssh.ParseRawPrivateKey(cert.Key)
	if err != nil {
		return err
	}

	return nil
}

// GenerateCACert generates a CA certificate, key and CSR based on the provided input.
func (ca *CA) GenerateCACert(input *CACSRInput) (*Certificate, error) {
	// prepare the certificate template
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName:         input.CommonName,
			Country:            []string{input.Country},
			Locality:           []string{input.Locality},
			Organization:       []string{input.Organization},
			OrganizationalUnit: []string{input.OrganizationUnit},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(input.Expiry),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// generate key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, input.KeySize)
	if err != nil {
		return nil, err
	}

	// create the certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	// convert Certificate into PEM format
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	// convert Private Key into PEM format
	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// create the clab certificate struct
	clabCert := &Certificate{
		Cert: caPEM.Bytes(),
		Key:  caPrivKeyPEM.Bytes(),
	}

	return clabCert, nil
}

// GenerateAndSignNodeCert generates and signs a node certificate, key and CSR based on the provided input and signs it with the CA.
func (ca *CA) GenerateAndSignNodeCert(input *NodeCSRInput) (*Certificate, error) {
	// parse hosts from input to retrieve dns and ip SANs
	dns, ip := parseHostsInput(input.Hosts)

	keysize := 2048
	if input.KeySize > 0 {
		keysize = input.KeySize
	}

	expiry := time.Until(time.Now().AddDate(1, 0, 0)) // 1 year as default
	if input.Expiry > 0 {
		expiry = input.Expiry
	}

	certTemplate := &x509.Certificate{
		RawSubject:   []byte{},
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:       []string{input.Organization},
			OrganizationalUnit: []string{input.OrganizationUnit},
			CommonName:         input.CommonName,
			Country:            []string{input.Country},
			Locality:           []string{input.Locality},
		},
		DNSNames:     dns,
		IPAddresses:  ip,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(expiry),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	newPrivKey, err := rsa.GenerateKey(rand.Reader, keysize)
	if err != nil {
		return nil, err
	}

	// create the certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, ca.cert, &newPrivKey.PublicKey, ca.key)
	if err != nil {
		return nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(newPrivKey),
	})

	// create the clab certificate struct
	clabCert := &Certificate{
		Cert: certPEM.Bytes(),
		Key:  certPrivKeyPEM.Bytes(),
	}

	return clabCert, nil
}

func parseHostsInput(hosts []string) ([]string, []net.IP) {
	var dns []string
	var ip []net.IP

	for _, host := range hosts {
		if net.ParseIP(host) != nil {
			ip = append(ip, net.ParseIP(host))
		} else {
			dns = append(dns, host)
		}
	}

	return dns, ip
}

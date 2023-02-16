// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"os"
	gopath "path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/cert/cfssl"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var (
	commonName       string
	country          string
	locality         string
	organization     string
	organizationUnit string
	expiry           string
	path             string
	caNamePrefix     string
	certNamePrefix   string
	certHosts        []string
	caCertPath       string
	caKeyPath        string
)

func init() {
	toolsCmd.AddCommand(certCmd)
	certCmd.AddCommand(CACmd)
	certCmd.AddCommand(signCertCmd)
	CACmd.AddCommand(CACreateCmd)

	CACreateCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.dev", "Common Name")
	CACreateCmd.Flags().StringVarP(&country, "c", "", "Internet", "Country")
	CACreateCmd.Flags().StringVarP(&locality, "l", "", "Server", "Location")
	CACreateCmd.Flags().StringVarP(&organization, "o", "", "Containerlab", "Organization")
	CACreateCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	CACreateCmd.Flags().StringVarP(&expiry, "expiry", "e", "87600h", "certificate validity period")
	CACreateCmd.Flags().StringVarP(&path, "path", "p", "",
		"path to write certificates to. Default is current working directory")
	CACreateCmd.Flags().StringVarP(&caNamePrefix, "name", "n", "root-ca", "certificate/key filename prefix")

	signCertCmd.Flags().StringSliceVarP(&certHosts, "hosts", "", []string{},
		"comma separate list of hosts of a certificate")
	signCertCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.dev", "Common Name")
	signCertCmd.Flags().StringVarP(&caCertPath, "ca-cert", "", "", "Path to CA certificate")
	signCertCmd.Flags().StringVarP(&caKeyPath, "ca-key", "", "", "Path to CA private key")
	signCertCmd.Flags().StringVarP(&country, "c", "", "Internet", "Country")
	signCertCmd.Flags().StringVarP(&locality, "l", "", "Server", "Location")
	signCertCmd.Flags().StringVarP(&organization, "o", "", "Containerlab", "Organization")
	signCertCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	signCertCmd.Flags().StringVarP(&path, "path", "p", "",
		"path to write certificate and key to. Default is current working directory")
	signCertCmd.Flags().StringVarP(&certNamePrefix, "name", "n", "cert", "certificate/key filename prefix")
}

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "TLS certificate operations",
}

var CACmd = &cobra.Command{
	Use:   "ca",
	Short: "certificate authority operations",
}

var CACreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create ca certificate and keys",
	RunE:  createCA,
}

var signCertCmd = &cobra.Command{
	Use:   "sign",
	Short: "create and sign certificate",
	RunE:  signCert,
}

// createCA creates a new CA certificate and key and writes them to the specified path.
func createCA(_ *cobra.Command, _ []string) error {
	var err error
	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	log.Infof("Certificate attributes: CN=%s, C=%s, L=%s, O=%s, OU=%s, Validity period=%s",
		commonName, country, locality, organization, organizationUnit, expiry)

	ca := cfssl.NewCA(nil, debug)

	csrInput := &cert.CACSRInput{
		CommonName:       commonName,
		Country:          country,
		Locality:         locality,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expiry,
	}

	caCert, err := ca.GenerateRootCert(csrInput)
	if err != nil {
		return err
	}

	utils.CreateDirectory(path, 0777)

	err = caCert.Write(
		gopath.Join(path, caNamePrefix+types.CertFileSuffix),
		gopath.Join(path, caNamePrefix+types.KeyFileSuffix),
		"",
	)
	if err != nil {
		return err
	}

	return nil
}

// create node certificate and sign it with CA.
func signCert(_ *cobra.Command, _ []string) error {
	var err error

	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	rootCa := cfssl.NewCA(nil, debug)

	var caCert *cert.Certificate
	log.Debugf("caCertPath: %q", caCertPath)
	if caCertPath != "" {
		// try loading the CA certificarte from disk via the explicite given path
		caCert, err = cert.LoadCertificateFromDisk(caCertPath, caKeyPath, "")
		if err != nil {
			return err
		}
	}

	// provide the root Cert to the CA
	err = rootCa.SetRootCertificate(caCert)
	if err != nil {
		return err
	}

	log.Infof("Creating and signing certificate: Hosts=%q, CN=%s, C=%s, L=%s, O=%s, OU=%s",
		certHosts, commonName, country, locality, organization, organizationUnit)

	nodeCert, err := rootCa.GenerateNodeCert(&cert.NodeCSRInput{
		Hosts:            certHosts,
		CommonName:       commonName,
		Country:          country,
		Locality:         locality,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expiry,
		Name:             certNamePrefix,
	})
	if err != nil {
		return err
	}

	if certNamePrefix == "" {
		certNamePrefix = certHosts[0]
	}

	utils.CreateDirectory(path, 0777)

	// store the cert
	err = nodeCert.Write(gopath.Join(path, certNamePrefix+".pem"), gopath.Join(path, certNamePrefix+".key"), gopath.Join(path, certNamePrefix+".csr"))
	if err != nil {
		return err
	}

	return nil
}

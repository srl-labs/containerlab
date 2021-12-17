// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"
	"text/template"

	cfssllog "github.com/cloudflare/cfssl/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab"
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

	CACreateCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.srlinux.dev", "Common Name")
	CACreateCmd.Flags().StringVarP(&country, "c", "", "Internet", "Country")
	CACreateCmd.Flags().StringVarP(&locality, "l", "", "Server", "Location")
	CACreateCmd.Flags().StringVarP(&organization, "o", "", "Containerlab", "Organization")
	CACreateCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	CACreateCmd.Flags().StringVarP(&expiry, "expiry", "e", "87600h", "certificate validity period")
	CACreateCmd.Flags().StringVarP(&path, "path", "p", "", "path to write certificates to. Default is current working directory")
	CACreateCmd.Flags().StringVarP(&caNamePrefix, "name", "n", "ca", "certificate/key filename prefix")

	signCertCmd.Flags().StringSliceVarP(&certHosts, "hosts", "", []string{}, "comma separate list of hosts of a certificate")
	signCertCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.srlinux.dev", "Common Name")
	signCertCmd.Flags().StringVarP(&caCertPath, "ca-cert", "", "", "Path to CA certificate")
	signCertCmd.Flags().StringVarP(&caKeyPath, "ca-key", "", "", "Path to CA private key")
	signCertCmd.Flags().StringVarP(&country, "c", "", "Internet", "Country")
	signCertCmd.Flags().StringVarP(&locality, "l", "", "Server", "Location")
	signCertCmd.Flags().StringVarP(&organization, "o", "", "Containerlab", "Organization")
	signCertCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	signCertCmd.Flags().StringVarP(&path, "path", "p", "", "path to write certificate and key to. Default is current working directory")
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

func createCA(_ *cobra.Command, _ []string) error {
	csr := `{
	"CN": "{{.CommonName}}",
	"key": {
		"algo": "rsa",
		"size": 2048
	},
	"names": [{
		"C": "{{.Country}}",
		"L": "{{.Locality}}",
		"O": "{{.Organization}}",
		"OU": "{{.OrganizationUnit}}"
	}],
	"ca": {
		"expiry": "{{.Expiry}}"
	}
}
`
	var err error
	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
	}
	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	cfssllog.Level = cfssllog.LevelError
	if debug {
		cfssllog.Level = cfssllog.LevelDebug
	}

	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	c.Dir = &clab.Directory{
		LabCARoot: path,
	}

	log.Infof("Certificate attributes: CN=%s, C=%s, L=%s, O=%s, OU=%s, Validity period=%s", commonName, country, locality, organization, organizationUnit, expiry)

	csrTpl, err := template.New("csr").Parse(csr)
	if err != nil {
		return err
	}

	_, err = cert.GenerateRootCa(c.Dir.LabCARoot, csrTpl, cert.CaRootInput{
		CommonName:       commonName,
		Country:          country,
		Locality:         locality,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expiry,
		NamePrefix:       caNamePrefix,
	},
	)
	if err != nil {
		return fmt.Errorf("failed to generate rootCa: %v", err)
	}
	return nil
}

// create node certificate and sign it with CA
func signCert(_ *cobra.Command, _ []string) error {
	csr := `{
		"CN": "{{.CommonName}}",
		"hosts": [
			{{- range $i, $e := .Hosts}}
			{{- if $i}},{{end}}
			"{{.}}"
			{{- end}}
		],
		"key": {
			"algo": "rsa",
			"size": 2048
		},
		"names": [{
			"C": "{{.Country}}",
			"L": "{{.Locality}}",
			"O": "{{.Organization}}",
			"OU": "{{.OrganizationUnit}}"
		}]
	}
	`
	var err error

	cfssllog.Level = cfssllog.LevelError
	if debug {
		cfssllog.Level = cfssllog.LevelDebug
	}

	// Check that CA path/key is set
	if caCertPath == "" {
		return fmt.Errorf("CA cert path not set")
	}
	if caKeyPath == "" {
		return fmt.Errorf("CA key path not set")
	}
	
	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	log.Infof("Creating and signing certificate: Hosts=%q, CN=%s, C=%s, L=%s, O=%s, OU=%s", certHosts, commonName, country, locality, organization, organizationUnit)

	csrTpl, err := template.New("csr").Parse(csr)
	if err != nil {
		return err
	}

	_, err = cert.GenerateCert(caCertPath, caKeyPath, csrTpl, cert.CertInput{
		Hosts:            certHosts,
		CommonName:       commonName,
		Country:          country,
		Locality:         locality,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expiry,
		Name:             certNamePrefix,
	},
		path,
	)
	if err != nil {
		return fmt.Errorf("failed to generate and sign certificate: %v", err)
	}

	return nil
}

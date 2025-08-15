// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcert "github.com/srl-labs/containerlab/cert"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
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
	keySize          int
)

func certCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "cert",
		Short: "TLS certificate operations",
	}

	c.AddCommand(CACmd)
	CACmd.AddCommand(CACreateCmd)
	CACreateCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.dev", "Common Name")
	CACreateCmd.Flags().StringVarP(&country, "country", "c", "Internet", "Country")
	CACreateCmd.Flags().StringVarP(&locality, "locality", "l", "Server", "Location")
	CACreateCmd.Flags().StringVarP(&organization, "organization", "o", "Containerlab", "Organization")
	CACreateCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	CACreateCmd.Flags().StringVarP(&expiry, "expiry", "e", "87600h", "certificate validity period")
	CACreateCmd.Flags().StringVarP(&path, "path", "p", "",
		"path to write certificate and key to. Default is current working directory")
	CACreateCmd.Flags().StringVarP(&caNamePrefix, "name", "n", "ca", "certificate/key filename prefix")

	c.AddCommand(signCertCmd)
	signCertCmd.Flags().StringSliceVarP(&certHosts, "hosts", "", []string{},
		"comma separate list of hosts of a certificate")
	signCertCmd.Flags().StringVarP(&commonName, "cn", "", "containerlab.dev", "Common Name")
	signCertCmd.Flags().StringVarP(&caCertPath, "ca-cert", "", "", "Path to CA certificate")
	signCertCmd.Flags().StringVarP(&caKeyPath, "ca-key", "", "", "Path to CA private key")
	signCertCmd.Flags().StringVarP(&country, "country", "c", "Internet", "Country")
	signCertCmd.Flags().StringVarP(&locality, "locality", "l", "Server", "Location")
	signCertCmd.Flags().StringVarP(&organization, "organization", "o", "Containerlab", "Organization")
	signCertCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Tools", "Organization Unit")
	signCertCmd.Flags().StringVarP(&path, "path", "p", "",
		"path to write certificate and key to. Default is current working directory")
	signCertCmd.Flags().StringVarP(&certNamePrefix, "name", "n", "cert", "certificate/key filename prefix")
	signCertCmd.Flags().IntVarP(&keySize, "key-size", "", 2048, "private key size")

	return c, nil
}

var CACmd = &cobra.Command{
	Use:   "ca",
	Short: "certificate authority operations",
}

var CACreateCmd = &cobra.Command{
	Use:   "create",
	Short: "create ca certificate and key",
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

	ca := clabcert.NewCA()

	expDuration, err := time.ParseDuration(expiry)
	if err != nil {
		return fmt.Errorf("failed parsing expiry %s", expiry)
	}

	csrInput := &clabcert.CACSRInput{
		CommonName:       commonName,
		Country:          country,
		Locality:         locality,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expDuration,
		KeySize:          keySize,
	}

	caCert, err := ca.GenerateCACert(csrInput)
	if err != nil {
		return err
	}

	clabutils.CreateDirectory(path, 0o777) // skipcq: GSC-G302

	err = caCert.Write(
		filepath.Join(path, caNamePrefix+clabtypes.CertFileSuffix),
		filepath.Join(path, caNamePrefix+clabtypes.KeyFileSuffix),
		"",
	)
	if err != nil {
		return err
	}

	return nil
}

// signCert creates node certificate and sign it with CA.
func signCert(_ *cobra.Command, _ []string) error {
	var err error

	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	ca := clabcert.NewCA()

	var caCert *clabcert.Certificate

	log.Debugf("CA cert path: %q", caCertPath)
	if caCertPath != "" {
		// we might also honor the External CA env vars here
		caCert, err = clabcert.NewCertificateFromFile(caCertPath, caKeyPath, "")
		if err != nil {
			return err
		}
	}

	// set loaded certificate to a CA and initialize a signer
	err = ca.SetCACert(caCert)
	if err != nil {
		return err
	}

	log.Info("Creating and signing certificate",
		"Hosts", certHosts,
		"CN", commonName,
		"C", country,
		"L", locality,
		"O", organization,
		"OU", organizationUnit,
	)

	expDuration, err := time.ParseDuration(expiry)
	if err != nil {
		return fmt.Errorf("failed parsing expiry %s", expiry)
	}

	nodeCert, err := ca.GenerateAndSignNodeCert(
		&clabcert.NodeCSRInput{
			Hosts:            certHosts,
			CommonName:       commonName,
			Country:          country,
			Locality:         locality,
			Organization:     organization,
			OrganizationUnit: organizationUnit,
			Expiry:           expDuration,
			KeySize:          keySize,
		})
	if err != nil {
		return err
	}

	clabutils.CreateDirectory(path, 0o777) // skipcq: GSC-G302

	err = nodeCert.Write(
		filepath.Join(path, certNamePrefix+clabtypes.CertFileSuffix),
		filepath.Join(path, certNamePrefix+clabtypes.KeyFileSuffix),
		filepath.Join(path, certNamePrefix+clabtypes.CSRFileSuffix))
	if err != nil {
		return err
	}

	return nil
}

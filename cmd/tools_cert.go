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
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func certCmd(o *Options) (*cobra.Command, error) { //nolint: funlen
	c := &cobra.Command{
		Use:   "cert",
		Short: "TLS certificate operations",
	}

	CACmd := &cobra.Command{
		Use:   "ca",
		Short: "certificate authority operations",
	}

	c.AddCommand(CACmd)

	CACreateCmd := &cobra.Command{
		Use:   "create",
		Short: "create ca certificate and key",
		RunE: func(_ *cobra.Command, _ []string) error {
			return createCA(o)
		},
	}

	CACmd.AddCommand(CACreateCmd)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.CommonName,
		"cn",
		"",
		o.ToolsCert.CommonName,
		"Common Name",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.Country,
		"country",
		"c",
		o.ToolsCert.Country,
		"Country",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.Locality,
		"locality",
		"l",
		o.ToolsCert.Locality,
		"Location",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.Organization,
		"organization",
		"o",
		o.ToolsCert.Organization,
		"Organization",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.OrganizationUnit,
		"ou",
		"",
		o.ToolsCert.OrganizationUnit,
		"Organization Unit",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.Expiry,
		"expiry",
		"e",
		o.ToolsCert.Expiry,
		"certificate validity period",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.Path,
		"path",
		"p",
		o.ToolsCert.Path,
		"path to write certificate and key to. Default is current working directory",
	)
	CACreateCmd.Flags().StringVarP(
		&o.ToolsCert.CANamePrefix,
		"name",
		"n",
		"ca",
		"certificate/key filename prefix",
	)

	signCertCmd := &cobra.Command{
		Use:   "sign",
		Short: "create and sign certificate",
		RunE: func(_ *cobra.Command, _ []string) error {
			return signCert(o)
		},
	}

	c.AddCommand(signCertCmd)
	signCertCmd.Flags().StringSliceVarP(
		&o.ToolsCert.CertHosts,
		"hosts",
		"", o.ToolsCert.CertHosts,
		"comma separate list of hosts of a certificate",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.CommonName,
		"cn",
		"",
		o.ToolsCert.CommonName,
		"Common Name",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.CACertPath,
		"ca-cert",
		"",
		o.ToolsCert.CACertPath,
		"Path to CA certificate",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.CAKeyPath,
		"ca-key",
		"",
		o.ToolsCert.CAKeyPath,
		"Path to CA private key",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.Country,
		"country",
		"c",
		o.ToolsCert.Country,
		"Country",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.Locality,
		"locality",
		"l",
		o.ToolsCert.Locality,
		"Location",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.Organization,
		"organization",
		"o",
		o.ToolsCert.Organization,
		"Organization",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.OrganizationUnit,
		"ou",
		"",
		o.ToolsCert.OrganizationUnit,
		"Organization Unit",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.Path,
		"path",
		"p",
		o.ToolsCert.Path,
		"path to write certificate and key to. Default is current working directory",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.CertNamePrefix,
		"name",
		"n",
		o.ToolsCert.CertNamePrefix,
		"certificate/key filename prefix",
	)
	signCertCmd.Flags().UintVarP(
		&o.ToolsCert.KeySize,
		"key-size",
		"",
		o.ToolsCert.KeySize,
		"private key size",
	)
	signCertCmd.Flags().StringVarP(
		&o.ToolsCert.TopologyFile,
		"topology",
		"",
		o.ToolsCert.TopologyFile,
		"path to topology file to generate certificates for all nodes with certificate.issue=true",
	)

	return c, nil
}

// createCA creates a new CA certificate and key and writes them to the specified path.
func createCA(o *Options) error {
	var err error
	if o.ToolsCert.Path == "" {
		o.ToolsCert.Path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	log.Infof(
		"Certificate attributes: CN=%s, C=%s, L=%s, O=%s, OU=%s, Validity period=%s",
		o.ToolsCert.CommonName,
		o.ToolsCert.Country,
		o.ToolsCert.Locality,
		o.ToolsCert.Organization,
		o.ToolsCert.OrganizationUnit,
		o.ToolsCert.Expiry,
	)

	ca := clabcert.NewCA()

	expDuration, err := time.ParseDuration(o.ToolsCert.Expiry)
	if err != nil {
		return fmt.Errorf("failed parsing expiry %s", o.ToolsCert.Expiry)
	}

	csrInput := &clabcert.CACSRInput{
		CommonName:       o.ToolsCert.CommonName,
		Country:          o.ToolsCert.Country,
		Locality:         o.ToolsCert.Locality,
		Organization:     o.ToolsCert.Organization,
		OrganizationUnit: o.ToolsCert.OrganizationUnit,
		Expiry:           expDuration,
		KeySize:          int(o.ToolsCert.KeySize),
	}

	caCert, err := ca.GenerateCACert(csrInput)
	if err != nil {
		return err
	}

	clabutils.CreateDirectory(
		o.ToolsCert.Path,
		clabconstants.PermissionsOpen,
	) // skipcq: GSC-G302

	err = caCert.Write(
		filepath.Join(o.ToolsCert.Path, o.ToolsCert.CANamePrefix+clabtypes.CertFileSuffix),
		filepath.Join(o.ToolsCert.Path, o.ToolsCert.CANamePrefix+clabtypes.KeyFileSuffix),
		"",
	)
	if err != nil {
		return err
	}

	return nil
}

// signCert creates node certificate and sign it with CA.
func signCert(o *Options) error {
	// If topology file is provided, use topology-based certificate generation
	if o.ToolsCert.TopologyFile != "" {
		return signCertFromTopology(o)
	}
	
	// Otherwise, use the original single certificate generation
	return signSingleCert(o)
}

// signSingleCert creates a single node certificate and signs it with CA.
func signSingleCert(o *Options) error {
	var err error

	if o.ToolsCert.Path == "" {
		o.ToolsCert.Path, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	ca := clabcert.NewCA()

	var caCert *clabcert.Certificate

	log.Debugf("CA cert path: %q", o.ToolsCert.CACertPath)

	if o.ToolsCert.CACertPath != "" {
		// we might also honor the External CA env vars here
		caCert, err = clabcert.NewCertificateFromFile(
			o.ToolsCert.CACertPath,
			o.ToolsCert.CAKeyPath,
			"",
		)
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
		"Hosts", o.ToolsCert.CertHosts,
		"CN", o.ToolsCert.CommonName,
		"C", o.ToolsCert.Country,
		"L", o.ToolsCert.Locality,
		"O", o.ToolsCert.Organization,
		"OU", o.ToolsCert.OrganizationUnit,
	)

	expDuration, err := time.ParseDuration(o.ToolsCert.Expiry)
	if err != nil {
		return fmt.Errorf("failed parsing expiry %s", o.ToolsCert.Expiry)
	}

	nodeCert, err := ca.GenerateAndSignNodeCert(
		&clabcert.NodeCSRInput{
			Hosts:            o.ToolsCert.CertHosts,
			CommonName:       o.ToolsCert.CommonName,
			Country:          o.ToolsCert.Country,
			Locality:         o.ToolsCert.Locality,
			Organization:     o.ToolsCert.Organization,
			OrganizationUnit: o.ToolsCert.OrganizationUnit,
			Expiry:           expDuration,
			KeySize:          int(o.ToolsCert.KeySize),
		})
	if err != nil {
		return err
	}

	clabutils.CreateDirectory(
		o.ToolsCert.Path,
		clabconstants.PermissionsOpen,
	) // skipcq: GSC-G302

	err = nodeCert.Write(
		filepath.Join(o.ToolsCert.Path, o.ToolsCert.CertNamePrefix+clabtypes.CertFileSuffix),
		filepath.Join(o.ToolsCert.Path, o.ToolsCert.CertNamePrefix+clabtypes.KeyFileSuffix),
		filepath.Join(o.ToolsCert.Path, o.ToolsCert.CertNamePrefix+clabtypes.CSRFileSuffix))
	if err != nil {
		return err
	}

	return nil
}

// signCertFromTopology generates certificates for all nodes in a topology file that have certificate.issue=true.
func signCertFromTopology(o *Options) error {
	// Load topology from file
	c, err := clabcore.NewContainerLab()
	if err != nil {
		return fmt.Errorf("failed to create containerlab instance: %w", err)
	}

	err = c.LoadTopologyFromFile(o.ToolsCert.TopologyFile, "")
	if err != nil {
		return fmt.Errorf("failed to load topology from file %s: %w", o.ToolsCert.TopologyFile, err)
	}

	// Initialize CA
	ca := clabcert.NewCA()
	var caCert *clabcert.Certificate

	// Load CA certificate if provided
	if o.ToolsCert.CACertPath != "" {
		caCert, err = clabcert.NewCertificateFromFile(
			o.ToolsCert.CACertPath,
			o.ToolsCert.CAKeyPath,
			"",
		)
		if err != nil {
			return fmt.Errorf("failed to load CA certificate: %w", err) 
		}
	} else {
		// Try to load CA from topology lab directory
		caCertPath := c.TopoPaths.CaCertAbsFilename()
		caKeyPath := c.TopoPaths.CaKeyAbsFilename()
		if _, err := os.Stat(caCertPath); err == nil {
			if _, err := os.Stat(caKeyPath); err == nil {
				caCert, err = clabcert.NewCertificateFromFile(caCertPath, caKeyPath, "")
				if err != nil {
					log.Warnf("Failed to load CA from topology directory: %v", err)
				}
			}
		}
	}

	// Set CA certificate
	err = ca.SetCACert(caCert)
	if err != nil {
		return fmt.Errorf("failed to set CA certificate: %w", err)
	}

	// Get default expiry duration
	expDuration, err := time.ParseDuration(o.ToolsCert.Expiry)
	if err != nil {
		return fmt.Errorf("failed parsing expiry %s: %w", o.ToolsCert.Expiry, err)
	}

	// Counter to track processed nodes
	processedNodes := 0

	// Iterate through topology nodes
	for nodeName, nodeDef := range c.Config.Topology.Nodes {
		// Get resolved certificate config for this node 
		certConfig := c.Config.Topology.GetCertificateConfig(nodeName)
		
		// Skip nodes that don't have certificate.issue=true
		if certConfig == nil || certConfig.Issue == nil || !*certConfig.Issue {
			log.Debugf("Skipping node %s: certificate generation not enabled", nodeName)
			continue
		}

		log.Infof("Generating certificate for node %s", nodeName)

		// Build hosts list similar to how nodes/default_node.go does it
		var longName string
		if c.Config.Prefix != nil {
			longName = fmt.Sprintf("%s-%s-%s", *c.Config.Prefix, c.Config.Name, nodeName)
		} else {
			longName = fmt.Sprintf("clab-%s-%s", c.Config.Name, nodeName)
		}

		hosts := []string{
			nodeName,
			longName,
			nodeName + "." + c.Config.Name + ".io",
		}

		// Add configured SANs
		if certConfig.SANs != nil {
			hosts = append(hosts, certConfig.SANs...)
		}

		// Add management IPs if defined
		if nodeDef.MgmtIPv4 != "" {
			hosts = append(hosts, nodeDef.MgmtIPv4)
		}
		if nodeDef.MgmtIPv6 != "" {
			hosts = append(hosts, nodeDef.MgmtIPv6)
		}

		// Determine certificate validity duration
		certExpiry := expDuration
		if certConfig.ValidityDuration > 0 {
			certExpiry = certConfig.ValidityDuration
		}

		// Determine key size
		keySize := int(o.ToolsCert.KeySize)
		if certConfig.KeySize > 0 {
			keySize = certConfig.KeySize
		}

		// Generate certificate
		nodeCert, err := ca.GenerateAndSignNodeCert(
			&clabcert.NodeCSRInput{
				CommonName:   nodeName + "." + c.Config.Name + ".io",
				Hosts:        hosts,
				Organization: "containerlab",
				Country:      "US",
				KeySize:      keySize,
				Expiry:       certExpiry,
			})
		if err != nil {
			return fmt.Errorf("failed to generate certificate for node %s: %w", nodeName, err)
		}

		// Create node certificate directory
		nodeCertDir := c.TopoPaths.NodeTLSDir(nodeName)
		clabutils.CreateDirectory(nodeCertDir, clabconstants.PermissionsOpen)

		// Write certificate files
		err = nodeCert.Write(
			c.TopoPaths.NodeCertAbsFilename(nodeName),
			c.TopoPaths.NodeCertKeyAbsFilename(nodeName),
			c.TopoPaths.NodeCertCSRAbsFilename(nodeName))
		if err != nil {
			return fmt.Errorf("failed to write certificate files for node %s: %w", nodeName, err)
		}

		log.Infof("Successfully generated certificate for node %s", nodeName)
		processedNodes++
	}

	if processedNodes == 0 {
		log.Warnf("No nodes found with certificate.issue=true in topology %s", o.ToolsCert.TopologyFile)
	} else {
		log.Infof("Successfully generated certificates for %d nodes", processedNodes)  
	}

	return nil
}

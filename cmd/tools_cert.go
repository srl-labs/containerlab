package cmd

import (
	"fmt"
	"os"
	"text/template"

	cfssllog "github.com/cloudflare/cfssl/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
)

var (
	commonName       string
	country          string
	location         string
	organization     string
	organizationUnit string
	expiry           string
	path             string
	namePrefix       string
)

func init() {
	toolsCmd.AddCommand(certCmd)
	certCmd.AddCommand(CACmd)
	CACmd.AddCommand(CACreateCmd)
	CACreateCmd.Flags().StringVarP(&commonName, "cn", "", "CA Containerlab", "Common Name")
	CACreateCmd.Flags().StringVarP(&country, "c", "", "Belgium", "Country")
	CACreateCmd.Flags().StringVarP(&location, "l", "", "Antwerp", "Location")
	CACreateCmd.Flags().StringVarP(&organization, "o", "", "Containerlab", "Organization")
	CACreateCmd.Flags().StringVarP(&organizationUnit, "ou", "", "Containerlab Unit", "Organization Unit")
	CACreateCmd.Flags().StringVarP(&expiry, "expiry", "e", "262800h", "certificate validity period")
	CACreateCmd.Flags().StringVarP(&path, "path", "p", "", "path to write certificates to. Default is current working directory")
	CACreateCmd.Flags().StringVarP(&namePrefix, "name", "n", "", "certificate/key filename prefix")
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

func createCA(cmd *cobra.Command, args []string) error {
	csr := `{
	"CN": "{{.CommonName}}",
	"key": {
		"algo": "rsa",
		"size": 2048
	},
	"names": [{
		"C": "{{.Country}}",
		"L": "{{.Location}}",
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
		clab.WithDebug(debug),
		clab.WithTimeout(timeout),
	}
	c := clab.NewContainerLab(opts...)

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

	log.Infof("Certificate attributes: CN=%s, C=%s, L=%s, O=%s, OU=%s, Validity period=%s", commonName, country, location, organization, organizationUnit, expiry)

	csrTpl, err := template.New("csr").Parse(csr)
	if err != nil {
		return err
	}

	_, err = c.GenerateRootCa(csrTpl, clab.CaRootInput{
		CommonName:       commonName,
		Country:          country,
		Location:         location,
		Organization:     organization,
		OrganizationUnit: organizationUnit,
		Expiry:           expiry,
		NamePrefix:       namePrefix,
	},
	)
	if err != nil {
		return fmt.Errorf("failed to generate rootCa: %v", err)
	}
	return nil
}

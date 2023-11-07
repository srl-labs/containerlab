package vr_sros

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	"golang.org/x/crypto/ssh"
	"golang.org/x/exp/slices"
)

// importing Default Config template at compile time
//
//go:embed ssh_keys.go.tpl
var SROSSSHKeysTemplate string

// filterSSHPubKeys returns marshalled SSH keys that match either of the
// provided key algorithms.
// Since SR OS supports only certain key types, we need to filter out the rest.
func (s *vrSROS) filterSSHPubKeys(sshKeyAlgos []string) []string {
	keyValues := make([]string, 0, len(s.sshPubKeys))

	for _, k := range s.sshPubKeys {
		if slices.Contains(sshKeyAlgos, k.Type()) {

			keyType := k.Type()
			keyString := string(ssh.MarshalAuthorizedKey(k))

			// Remove the key type prefix
			keyString = strings.TrimPrefix(keyString, keyType+" ")

			// Remove the suffix (usually a comment or username)
			parts := strings.Fields(keyString)
			if len(parts) > 1 {
				keyString = parts[0]
			}

			keyValues = append(keyValues, keyString)
		}
	}

	return keyValues
}

// SROSTemplateData holds ssh keys for template generation.
type SROSTemplateData struct {
	SSHPubKeysRSA   []string
	SSHPubKeysECDSA []string
}

// configureSSHPublicKeys cofigures public keys extracted from clab host
// on SR OS node using SSH.
func (s *vrSROS) configureSSHPublicKeys(
	ctx context.Context, addr, platformName,
	username, password string, pubKeys []ssh.PublicKey) error {
	tplData := SROSTemplateData{}

	// creating a list of ECDSA Key Types
	rsaKeyTypes := []string{"ssh-rsa"}
	ecdsaKeyTypes := []string{"ssh-ed25519", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521"}

	// checking on existence of SSH Public Keys and populating tplData
	tplData.SSHPubKeysRSA = s.filterSSHPubKeys(rsaKeyTypes)
	tplData.SSHPubKeysECDSA = s.filterSSHPubKeys(ecdsaKeyTypes)

	t, err := template.New("SSHKeys").Funcs(
		gomplate.CreateFuncs(context.Background(), new(data.Data))).Parse(SROSSSHKeysTemplate)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, tplData)
	if err != nil {
		return err
	}

	err = s.applyPartialConfig(ctx, s.Cfg.MgmtIPv4Address, scrapliPlatformName,
		defaultCredentials.GetUsername(), defaultCredentials.GetPassword(),
		buf,
	)

	return err
}

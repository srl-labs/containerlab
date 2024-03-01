package vr_sros

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// SROSSSHKeysTemplate holds the template for the SSH keys configuration.
//
//go:embed ssh_keys.go.tpl
var SROSSSHKeysTemplate string

// generateSSHPublicKeysConfig generates public keys configuration blob
// to add keys extracted from the clab host.
func (s *vrSROS) generateSSHPublicKeysConfig() (io.Reader, error) {
	tplData := SROSTemplateData{}

	s.prepareSSHPubKeys(&tplData)

	t, err := template.New("SSHKeys").Funcs(
		gomplate.CreateFuncs(context.Background(), new(data.Data))).Parse(SROSSSHKeysTemplate)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, tplData)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// prepareSSHPubKeys maps the ssh pub keys into the SSH key type based slice
// and checks that not more then 32 keys per type are present, otherwise truncates
// the slices since SROS allows a max of 32 public keys per algorithm.
func (s *vrSROS) prepareSSHPubKeys(tplData *SROSTemplateData) {
	// a map of supported SSH key algorithms and the template slices
	// the keys should be added to.
	// In mapSSHPubKeys we map supported SSH key algorithms to the template slices.
	supportedSSHKeyAlgos := map[string]*[]string{
		ssh.KeyAlgoRSA:      &tplData.SSHPubKeysRSA,
		ssh.KeyAlgoECDSA521: &tplData.SSHPubKeysECDSA,
		ssh.KeyAlgoECDSA384: &tplData.SSHPubKeysECDSA,
		ssh.KeyAlgoECDSA256: &tplData.SSHPubKeysECDSA,
	}

	s.mapSSHPubKeys(supportedSSHKeyAlgos)

	if len(tplData.SSHPubKeysRSA) > 32 {
		log.Warnf("more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type")
		tplData.SSHPubKeysRSA = tplData.SSHPubKeysRSA[:32]
	}

	if len(tplData.SSHPubKeysECDSA) > 32 {
		log.Warnf("more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type")
		tplData.SSHPubKeysECDSA = tplData.SSHPubKeysECDSA[:32]
	}
}

// mapSSHPubKeys goes over s.sshPubKeys and puts the supported keys to the corresponding
// slices associated with the supported SSH key algorithms.
// supportedSSHKeyAlgos key is a SSH key algorithm and the value is a pointer to the slice
// that is used to store the keys of the corresponding algorithm family.
// Two slices are used to store RSA and ECDSA keys separately.
// The slices are modified in place by reference, so no return values are needed.
func (s *vrSROS) mapSSHPubKeys(supportedSSHKeyAlgos map[string]*[]string) {
	for _, k := range s.sshPubKeys {
		sshKeys, ok := supportedSSHKeyAlgos[k.Type()]
		if !ok {
			log.Debugf("unsupported SSH Key Algo %q, skipping key", k.Type())
			continue
		}

		// extract the fields
		// <keytype> <key> <comment>
		keyFields := strings.Fields(string(ssh.MarshalAuthorizedKey(k)))

		*sshKeys = append(*sshKeys, keyFields[1])
	}
}

package vr_sros

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// importing Default Config template at compile time
//
//go:embed ssh_keys.go.tpl
var SROSSSHKeysTemplate string

// filterSSHPubKeys returns rsa and ecdsa keys from the list of ssh keys stored at s.sshPubKeys.
// Since SR OS supports only certain key types, we need to filter out the rest.
func (s *vrSROS) filterSSHPubKeys(supportedSSHKeyAlgos map[string]struct{}) (rsaKeys []string, ecdsaKeys []string) {
	rsaKeys = make([]string, 0, len(s.sshPubKeys))
	ecdsaKeys = make([]string, 0, len(s.sshPubKeys))

	for _, k := range s.sshPubKeys {
		if _, ok := supportedSSHKeyAlgos[k.Type()]; !ok {
			log.Debugf("unsupported SSH Key Algo %q, skipping key", k.Type())
			continue
		}

		// extract the fields
		// <keytype> <key> <comment>
		keyFields := strings.Fields(string(ssh.MarshalAuthorizedKey(k)))

		switch {
		case isRSAKey(k):
			rsaKeys = append(rsaKeys, keyFields[1])
		case isECDSAKey(k):
			ecdsaKeys = append(ecdsaKeys, keyFields[1])
		}
	}

	return rsaKeys, ecdsaKeys
}

func isRSAKey(key ssh.PublicKey) bool {
	return key.Type() == ssh.KeyAlgoRSA
}

func isECDSAKey(key ssh.PublicKey) bool {
	kType := key.Type()

	return kType == ssh.KeyAlgoECDSA521 ||
		kType == ssh.KeyAlgoECDSA384 ||
		kType == ssh.KeyAlgoECDSA256
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

	// a map of supported SSH key algorithms
	supportedSSHKeyAlgos := map[string]struct{}{
		ssh.KeyAlgoRSA:      {},
		ssh.KeyAlgoECDSA521: {},
		ssh.KeyAlgoECDSA384: {},
		ssh.KeyAlgoECDSA256: {},
	}

	tplData.SSHPubKeysRSA, tplData.SSHPubKeysECDSA = s.filterSSHPubKeys(supportedSSHKeyAlgos)

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

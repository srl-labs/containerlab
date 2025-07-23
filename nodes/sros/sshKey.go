package sros

import (
	_ "embed"
	"strings"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"
)

// prepareSSHPubKeys maps the ssh pub keys into the SSH key type based slice
// and checks that not more then 32 keys per type are present, otherwise truncates
// the slices since SROS allows a max of 32 public keys per algorithm.
func (n *sros) prepareSSHPubKeys(tplData *srosTemplateData) {
	// a map of supported SSH key algorithms and the template slices
	// the keys should be added to.
	// In mapSSHPubKeys we map supported SSH key algorithms to the template slices.
	supportedSSHKeyAlgos := map[string]*[]string{
		ssh.KeyAlgoRSA:      &tplData.SSHPubKeysRSA,
		ssh.KeyAlgoECDSA521: &tplData.SSHPubKeysECDSA,
		ssh.KeyAlgoECDSA384: &tplData.SSHPubKeysECDSA,
		ssh.KeyAlgoECDSA256: &tplData.SSHPubKeysECDSA,
	}

	n.mapSSHPubKeys(supportedSSHKeyAlgos)

	if len(tplData.SSHPubKeysRSA) > 32 {
		log.Warn("more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type")
		tplData.SSHPubKeysRSA = tplData.SSHPubKeysRSA[:32]
	}

	if len(tplData.SSHPubKeysECDSA) > 32 {
		log.Warn("more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type")
		tplData.SSHPubKeysECDSA = tplData.SSHPubKeysECDSA[:32]
	}
}

// mapSSHPubKeys goes over s.sshPubKeys and puts the supported keys to the corresponding
// slices associated with the supported SSH key algorithms.
// supportedSSHKeyAlgos key is a SSH key algorithm and the value is a pointer to the slice
// that is used to store the keys of the corresponding algorithm family.
// Two slices are used to store RSA and ECDSA keys separately.
// The slices are modified in place by reference, so no return values are needed.
func (n *sros) mapSSHPubKeys(supportedSSHKeyAlgos map[string]*[]string) {
	for _, k := range n.sshPubKeys {
		sshKeys, ok := supportedSSHKeyAlgos[k.Type()]
		if !ok {
			log.Debug("Unsupported SSH Key Algo, skipping key", "node", n.Cfg.ShortName,
				"key", string(ssh.MarshalAuthorizedKey(k)))
			continue
		}

		// extract the fields
		// <keytype> <key> <comment>
		keyFields := strings.Fields(string(ssh.MarshalAuthorizedKey(k)))

		*sshKeys = append(*sshKeys, keyFields[1])
	}
}

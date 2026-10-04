package sros

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	scrapligocli "github.com/scrapli/scrapligo/v2/cli"
	scrapligologging "github.com/scrapli/scrapligo/v2/logging"
	scrapligooptions "github.com/scrapli/scrapligo/v2/options"
	"golang.org/x/crypto/ssh"
	"golang.org/x/mod/semver"
)

// limitSSHKeys truncates SSH keys to SROS maximum of 32 per type.
func limitSSHKeys(keys *[]string, keyType string) {
	if len(*keys) > 32 {
		log.Warnf(
			"More than 32 public %s SSH keys found on the system. Selecting first 32 keys since SROS supports max 32 per key type",
			keyType,
		)
		*keys = (*keys)[:32]
	}
}

// prepareSSHPubKeys maps the ssh pub keys into the SSH key type based slice
// and checks that not more than 32 keys per type are present, otherwise truncates
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

	currVersion := tplData.SwVersion.MajorMinorSemverString()
	if semver.Compare(currVersion, "v26.7") >= 0 {
		supportedSSHKeyAlgos[ssh.KeyAlgoED25519] = &tplData.SSHPubKeysED25519
	}

	n.mapSSHPubKeys(supportedSSHKeyAlgos)

	limitSSHKeys(&tplData.SSHPubKeysRSA, "RSA")
	limitSSHKeys(&tplData.SSHPubKeysECDSA, "ECDSA")
	if semver.Compare(currVersion, "v26.7") >= 0 {
		limitSSHKeys(&tplData.SSHPubKeysED25519, "ED25519")
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

const sshPort = 22

func (n *sros) srosSendCommandsSSH(
	ctx context.Context,
	scrapli_platform string,
	cmds []string,
) error {
	addr, err := n.MgmtIPAddr()
	if err != nil {
		return err
	}
	sl := log.StandardLog(log.StandardLogOptions{
		ForceLevel: log.DebugLevel,
	})

	opts := []scrapligooptions.Option{
		scrapligooptions.WithDefinitionFileOrName(scrapli_platform),
		scrapligooptions.WithPort(sshPort),
		scrapligooptions.WithUsername(n.Cfg.Credentials.Username),
		scrapligooptions.WithPassword(n.Cfg.Credentials.Password),
		scrapligooptions.WithTransportSSH2(),
		scrapligooptions.WithOperationTimeout(5 * time.Second),
		scrapligooptions.WithLogger(func(level scrapligologging.LogLevel, message string) {
			sl.Print(message)
		}),
		scrapligooptions.WithLoggerLevel(scrapligologging.Debug),
	}

	c, err := scrapligocli.NewCli(addr, opts...)
	if err != nil {
		return fmt.Errorf("%q-%q: failed to create cli session: %+v", n.Cfg.ShortName, addr, err)
	}

	if _, err := c.Open(ctx); err != nil {
		return fmt.Errorf("%q failed to open ssh2/cli session; error: %+v", n.Cfg.ShortName, err)
	}
	defer c.Close(ctx)

	res, err := c.SendInputs(ctx, cmds)
	if err != nil {
		return fmt.Errorf("failed to send command: %+v", err)
	}
	if res.Failed() {
		return fmt.Errorf("failed to send command (failed result: %s)", res.Result())
	}

	log.Debug(
		"Saved running configuration",
		"node",
		n.Cfg.ShortName,
		"addr",
		addr,
		"config-mode",
		n.Cfg.Env[envSrosConfigMode],
	)
	return nil
}

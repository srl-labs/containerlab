package sros

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/options"
	scraplilogging "github.com/scrapli/scrapligo/logging"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
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
		log.Warn(
			"more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type",
		)
		tplData.SSHPubKeysRSA = tplData.SSHPubKeysRSA[:32]
	}

	if len(tplData.SSHPubKeysECDSA) > 32 {
		log.Warn(
			"more then 32 public RSA ssh keys found on the system. Selecting first 32 keys since SROS supports max. 32 per key type",
		)
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

func (n *sros) srosSendCommandsSSH(_ context.Context, scrapli_platform string, c []string) error {
	addr, err := n.MgmtIPAddr()
	if err != nil {
		return err
	}
	sl := log.StandardLog(log.StandardLogOptions{
		ForceLevel: log.DebugLevel,
	})
	li, err := scraplilogging.NewInstance(
		scraplilogging.WithLevel("debug"),
		scraplilogging.WithLogger(sl.Print))
	if err != nil {
		return err
	}

	opts := []util.Option{
		options.WithAuthNoStrictKey(),
		options.WithAuthUsername(defaultCredentials.GetUsername()),
		options.WithAuthPassword(defaultCredentials.GetPassword()),
		options.WithTransportType(transport.StandardTransport),
		options.WithTimeoutOps(5 * time.Second),
		options.WithLogger(li),
	}
	p, err := platform.NewPlatform(scrapli_platform, fmt.Sprintf("[%s]", addr), opts...)
	if err != nil {
		return fmt.Errorf("%q-%q: failed to create platform: %+v", n.Cfg.ShortName, addr, err)
	}

	d, err := p.GetNetworkDriver()
	if err != nil {
		return fmt.Errorf("%q-%q: could not create the driver: %+v", n.Cfg.ShortName, addr, err)
	}
	if err := d.Open(); err != nil {
		return fmt.Errorf("%q failed to open ssh2/cli session; error: %+v", n.Cfg.ShortName, err)
	}
	defer d.Close()
	mresp, err := d.SendCommands(c)
	if err != nil || (mresp != nil && mresp.Failed != nil) {
		if mresp != nil {
			return fmt.Errorf("failed to send command; error: %+v %+v", err, mresp.Failed)
		} else {
			return fmt.Errorf("failed to send command; error: %+v", err)
		}
	}
	log.Debug("Saved running configuration", "node", n.Cfg.ShortName, "addr", addr, "config-mode", n.Cfg.Env[envSrosConfigMode])
	return nil
}

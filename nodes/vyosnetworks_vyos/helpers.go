package vyosnetworks_vyos

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/scrapli/scrapligo/driver/network"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/steiler/acls"
)

func (n *vyos) save(_ context.Context, cli *network.Driver) error {
	log.Debug("Saving config", "node", n.Cfg.ShortName)
	resp, err := cli.SendConfigs(saveCmd)
	if err != nil {
		return err
	} else if resp.Failed != nil {
		return fmt.Errorf("save failed. Response: %w", err)
	}
	log.Info("Save successful", "node", n.Cfg.ShortName)
	return nil
}

func (n *vyos) newCli() (*network.Driver, error) {
	cli, err := clabutils.SpawnCLIviaExec(
		scrapliPlatformName,
		n.Cfg.LongName,
		n.Runtime.GetName())
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (n *vyos) createVyosFiles(_ context.Context) error {
	nodeCfg := n.Config()

	// generate config dir
	clabutils.CreateDirectory(n.configDir, clabconstants.PermissionsOpen)
	log.Debugf("Chowning dir %s", n.configDir)
	if err := os.Chown(n.Cfg.LabDir, 0, vyattacfg_gid); err != nil {
		return err
	}

	nodeCfg.MgmtIntf = "eth0"

	// use startup config file provided by a user
	if nodeCfg.StartupConfig != "" {
		c, err := os.ReadFile(nodeCfg.StartupConfig)
		if err != nil {
			return err
		}
		cfgTemplate = string(c)
	}

	err := n.GenerateConfig(nodeCfg.ResStartupConfig, cfgTemplate)
	if err != nil {
		return err
	}

	// Vyos needs pre and post boot config scripts to be present even if they're noops
	scriptDir := filepath.Join(n.configDir, "scripts")
	preScript := filepath.Join(scriptDir, "vyos-preconfig-bootup.script")
	postScript := filepath.Join(scriptDir, "vyos-postconfig-bootup.script")
	clabutils.CreateDirectory(scriptDir, clabconstants.PermissionsOpen)

	for _, s := range []string{preScript, postScript} {
		clabutils.CreateFile(s, "#!/bin/sh")
		os.Chmod(s, clabconstants.PermissionsDirDefault)
		os.Chown(s, 0, vyattacfg_gid)
	}

	return nil
}

// clab applies ACLs to the node directory based on the idea that the NOS
// parses everything through a management system. While Vyos does have a
// management system it does a bunch of stuff as a regular linux user so the
// directory needs rw access for the vyattacfg group.
func (n *vyos) fixdirACL() error {
	log.Debugf("Setting up %s ACLs", n.Cfg.LabDir)
	a := &acls.ACL{}
	if err := a.Load(n.Cfg.LabDir, acls.PosixACLAccess); err != nil {
		return err
	}
	entry := acls.NewEntry(acls.TAG_ACL_GROUP, uint32(vyattacfg_gid), 7)
	if err := a.AddEntry(entry); err != nil {
		return err
	}
	if err := a.Apply(n.Cfg.LabDir, acls.PosixACLAccess); err != nil {
		return err
	}
	if err := a.Apply(n.Cfg.LabDir, acls.PosixACLDefault); err != nil {
		return err
	}

	return nil
}

// Convert the PKCS#1 formatted key that clab generates into a PKCS#8 that Vyos
// requires.
func pkcs1To8(der []byte) ([]byte, error) {
	// the key we get is a []byte but it's actually a pem string not the raw data
	log.Debug("Converting PKCS#1 key to PKCS#8")
	block, _ := pem.Decode(der)
	if block == nil {
		return nil, errors.New("something went wrong decoding the PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	p8Key, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	p8Key = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: p8Key,
	})

	return p8Key, nil
}

func (n *vyos) authorizedKeyCmds() []string {
	var cmds []string
	var b strings.Builder
	baseCmd := fmt.Sprintf("set system login user %s authentication public-keys clab ", n.creds.GetUsername())

	for _, k := range n.SSHPubKeys {
		// Set ssh key type
		b.WriteString(baseCmd)
		b.WriteString("type ")
		b.WriteString(k.Type())
		cmds = append(cmds, b.String())
		b.Reset()
		b.WriteString(baseCmd)
		b.WriteString("key ")
		b.WriteString(base64.StdEncoding.EncodeToString(k.Marshal()))
		cmds = append(cmds, b.String())
		b.Reset()
	}

	return cmds
}

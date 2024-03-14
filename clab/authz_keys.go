// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	pubKeysGlob = "~/.ssh/*.pub"
	// authorized keys file path on a clab host that is used to create the clabAuthzKeys file.
	authzKeysFPath = "~/.ssh/authorized_keys"
)

// createAuthzKeysFile creates the authorized_keys file in the lab directory
// using the public ssh keys retrieved from agent and local files.
func (c *CLab) createAuthzKeysFile() error {
	b := new(bytes.Buffer)

	for _, k := range c.sSHPubKeys {
		x := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
		addKeyToBuffer(b, x)
	}

	clabAuthzKeysFPath := c.TopoPaths.AuthorizedKeysFilename()
	if err := utils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(clabAuthzKeysFPath, 0644) // skipcq: GSC-G302
}

// RetrieveSSHPubKeysFromFiles retrieves public keys from the ~/.ssh/*.authorized_keys
// and ~/.ssh/*.pub files.
func RetrieveSSHPubKeysFromFiles() ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	p := utils.ResolvePath(pubKeysGlob, "")

	all, err := filepath.Glob(p)
	if err != nil {
		return nil, fmt.Errorf("failed globbing the path %s", p)
	}

	f := utils.ResolvePath(authzKeysFPath, "")

	if utils.FileExists(f) {
		log.Debugf("%s found, adding it to the list of files to get public keys from", f)
		all = append(all, f)
	}

	keys, err = utils.LoadSSHPubKeysFromFiles(all)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// retrieveSSHPubKeys retrieves the PubKeys from the different sources
// SSHAgent as well as all home dir based /.ssh/*.pub files.
func (c *CLab) retrieveSSHPubKeys() ([]ssh.PublicKey, error) {
	keys := make([]ssh.PublicKey, 0)

	var errs error

	// any errors encountered during the retrieval of the keys are not fatal
	// we accumulate them and log.
	fkeys, err := RetrieveSSHPubKeysFromFiles()
	if err != nil {
		errs = errors.Join(err)
	}

	agentKeys, err := RetrieveSSHAgentKeys()
	if err != nil {
		errs = errors.Join(err)
	}

	keysM := map[string]ssh.PublicKey{}
	for _, k := range append(fkeys, agentKeys...) {
		keysM[string(ssh.MarshalAuthorizedKey(k))] = k
	}

	for _, k := range keysM {
		keys = append(keys, k)
	}

	return keys, errs
}

// addKeyToBuffer adds a key to the buffer if the key is not already present.
func addKeyToBuffer(b *bytes.Buffer, key string) {
	if !strings.Contains(b.String(), key) {
		b.WriteString(key + "\n")
	}
}

// RetrieveSSHAgentKeys retrieves public keys registered with the ssh-agent.
func RetrieveSSHAgentKeys() ([]ssh.PublicKey, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if len(socket) == 0 {
		log.Debug("SSH_AUTH_SOCK not set, skipping pubkey fetching")
		return nil, nil
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH_AUTH_SOCK: %w", err)
	}

	agentClient := agent.NewClient(conn)
	keys, err := agentClient.List()
	if err != nil {
		return nil, fmt.Errorf("error listing agent's pub keys %w", err)
	}

	log.Debugf("extracted %d keys from ssh-agent", len(keys))

	var pubKeys []ssh.PublicKey

	for _, key := range keys {
		pkey, err := ssh.ParsePublicKey(key.Blob)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, pkey)
	}

	return pubKeys, nil
}

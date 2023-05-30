// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh/agent"
)

const (
	pubKeysGlob = "~/.ssh/*.pub"
	// authorized keys file path on a clab host that is used to create the clabAuthzKeys file.
	authzKeysFPath = "~/.ssh/authorized_keys"
)

// CreateAuthzKeysFile creats the authorized_keys file in the lab directory
// if any files ~/.ssh/*.pub found.
func (c *CLab) CreateAuthzKeysFile() error {
	b := new(bytes.Buffer)

	p := utils.ResolvePath(pubKeysGlob, "")

	all, err := filepath.Glob(p)
	if err != nil {
		return fmt.Errorf("failed globbing the path %s", p)
	}

	f := utils.ResolvePath(authzKeysFPath, "")

	if utils.FileExists(f) {
		log.Debugf("%s found, adding the public keys it contains", f)
		all = append(all, f)
	}

	// get keys registered with ssh-agent
	keys, err := SSHAgentKeys()
	if err != nil {
		log.Debug(err)
	}

	log.Debugf("extracted %d keys from ssh-agent", len(keys))
	for _, k := range keys {
		addKeyToBuffer(b, k)
	}

	for _, fn := range all {
		rb, err := os.ReadFile(fn)
		if err != nil {
			return fmt.Errorf("failed reading the file %s: %v", fn, err)
		}

		addKeyToBuffer(b, string(rb))
	}

	clabAuthzKeysFPath := c.TopoPaths.AuthorizedKeysFilename()
	if err := utils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(clabAuthzKeysFPath, 0644) // skipcq: GSC-G302
}

// addKeyToBuffer adds a key to the buffer if the key is not already present.
func addKeyToBuffer(b *bytes.Buffer, key string) {
	// since they key might have a comment as a third field, we need to strip it
	elems := strings.Fields(key)
	if len(elems) < 2 {
		return
	}

	if !strings.Contains(b.String(), elems[1]) {
		b.WriteString(key + "\n")
	}
}

// SSHAgentKeys retrieves public keys registered with the ssh-agent.
func SSHAgentKeys() ([]string, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if len(socket) == 0 {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set, skipping pubkey fetching")
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

	var pubKeys []string
	for _, key := range keys {
		pubKeys = append(pubKeys, key.String())
	}

	return pubKeys, nil
}

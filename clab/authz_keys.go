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

	p := utils.ResolvePath(pubKeysGlob, c.TopoPaths.TopologyFileDir())

	all, err := filepath.Glob(p)
	if err != nil {
		return fmt.Errorf("failed globbing the path %s", p)
	}

	f := utils.ResolvePath(authzKeysFPath, c.TopoPaths.TopologyFileDir())

	if utils.FileExists(f) {
		log.Debugf("%s found, adding the public keys it contains", f)
		all = append(all, f)
	}

	// try extracting keys from ssh agent
	keys, err := retrieveAgentKeys()
	if err != nil {
		log.Debug(err)
	} else {
		log.Debugf("extracted %d keys from ssh-agent", len(keys))
		for _, k := range keys {
			b.WriteString(k + "\n")
		}
	}

	for _, fn := range all {
		rb, _ := os.ReadFile(fn)
		b.Write(rb)
	}

	clabAuthzKeysFPath := c.TopoPaths.AuthorizedKeysFilename()
	if err := utils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(clabAuthzKeysFPath, 0644) // skipcq: GSC-G302
}

// retrieveAgentKeys retrieves SSH Pubkeys from the ssh-agent
func retrieveAgentKeys() ([]string, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if len(socket) == 0 {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set skipping pubkey evaluation")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH_AUTH_SOCK: %w", err)
	}

	agentClient := agent.NewClient(conn)
	keys, err := agentClient.List()
	if err != nil {
		return nil, fmt.Errorf("error listing agent pub keys %w", err)
	}
	var pubKeys []string
	for _, key := range keys {
		pubKeys = append(pubKeys, key.String())
	}
	return pubKeys, nil
}

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
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/crypto/ssh"
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

	// get keys registered with ssh-agent
	keys, err := RetrieveSSHPubKeys()
	if err != nil {
		log.Debug(err)
	}

	for _, k := range keys {
		x := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k.PublicKey)))
		addKeyToBuffer(b, x)
	}

	clabAuthzKeysFPath := c.TopoPaths.AuthorizedKeysFilename()
	if err := utils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(clabAuthzKeysFPath, 0644) // skipcq: GSC-G302
}

// RetrieveSSHAuthorizedKeys retrieves public keys present in the pubKeysGlob
// wildcard path.
func RetrieveSSHAuthorizedKeys() ([]*types.SSHPubKey, error) {
	keys := []*types.SSHPubKey{}
	p := utils.ResolvePath(pubKeysGlob, "")

	all, err := filepath.Glob(p)
	if err != nil {
		return nil, fmt.Errorf("failed globbing the path %s", p)
	}

	f := utils.ResolvePath(authzKeysFPath, "")

	if utils.FileExists(f) {
		log.Debugf("%s found, adding the public keys it contains", f)
		all = append(all, f)
	}

	// iterate through all the *.pub files an parse them as ssh.PublicKey
	for _, fn := range all {
		rb, err := os.ReadFile(fn)
		if err != nil {
			return nil, fmt.Errorf("failed reading the file %s: %v", fn, err)
		}

		pubKey, comment, _, _, err := ssh.ParseAuthorizedKey(rb)
		if err != nil {
			return nil, err
		}

		keys = append(keys, types.NewSSHPublicKey(pubKey, comment))
	}
	return keys, nil
}

// RetrieveSSHPubKeys retrieves the PubKeys from the different sources
// SSHAgent as well as all home dir based /.ssh/*.pub files.
func RetrieveSSHPubKeys() ([]*types.SSHPubKey, error) {
	keys, err := RetrieveSSHAuthorizedKeys()
	if err != nil {
		return nil, err
	}

	agentKeys, err := RetrieveSSHAgentKeys()
	if err != nil {
		return nil, err
	}

	keys = append(keys, agentKeys...)

	return dedupKeys(keys), nil
}

func dedupKeys(keys []*types.SSHPubKey) []*types.SSHPubKey {
	if len(keys) <= 1 {
		return keys
	}
	result := make([]*types.SSHPubKey, 0, len(keys))

	// iterate through keys
	for idx, key := range keys {
		dupFound := false
		// keys to compare are the once greater then the index of the actual key
		// all others have already been compared via previous outer loop runs
		for i := idx + 1; i < len(keys); i++ {
			if key.Equals(keys[i]) {
				// if key is a duplicate one, indicate via dupFound and break
				// the result is, that the last instance of an x times duplicate key
				// will be added, since in the remaining list of keys no duplicate
				// will be found any more
				dupFound = true
				break
			}
		}
		// if no dup was found, add it to the result list
		if !dupFound {
			result = append(result, key)
		}
	}
	return result
}

// addKeyToBuffer adds a key to the buffer if the key is not already present.
func addKeyToBuffer(b *bytes.Buffer, key string) {
	if !strings.Contains(b.String(), key) {
		b.WriteString(key + "\n")
	}
}

// RetrieveSSHAgentKeys retrieves public keys registered with the ssh-agent.
func RetrieveSSHAgentKeys() ([]*types.SSHPubKey, error) {
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

	log.Debugf("extracted %d keys from ssh-agent", len(keys))

	var pubKeys []*types.SSHPubKey
	for _, key := range keys {
		pkey, err := ssh.ParsePublicKey(key.Blob)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, types.NewSSHPublicKey(pkey, key.Comment))
	}

	return pubKeys, nil
}

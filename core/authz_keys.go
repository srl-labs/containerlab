// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
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

	for _, k := range c.SSHPubKeys {
		x := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
		addKeyToBuffer(b, x)
	}

	clabAuthzKeysFPath := c.TopoPaths.AuthorizedKeysFilename()
	if err := clabutils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(
		clabAuthzKeysFPath,
		clabconstants.PermissionsFileDefault,
	) // skipcq: GSC-G302
}

// RetrieveSSHPubKeysFromFiles retrieves public keys from the ~/.ssh/*.authorized_keys
// and ~/.ssh/*.pub files.
func RetrieveSSHPubKeysFromFiles() ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey

	p := clabutils.ResolvePath(pubKeysGlob, "")

	all, err := filepath.Glob(p)
	if err != nil {
		return nil, fmt.Errorf("failed globbing the path %s", p)
	}

	f := clabutils.ResolvePath(authzKeysFPath, "")

	if clabutils.FileExists(f) {
		log.Debugf("%s found, adding it to the list of files to get public keys from", f)
		all = append(all, f)
	}

	keys, err = clabutils.LoadSSHPubKeysFromFiles(all)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// RetrieveSSHPubKeys retrieves the PubKeys from the different sources
// SSHAgent as well as all home dir based /.ssh/*.pub files.
func (c *CLab) RetrieveSSHPubKeys(ctx context.Context) ([]ssh.PublicKey, error) {
	keys := make([]ssh.PublicKey, 0)

	var errs error

	// any errors encountered during the retrieval of the keys are not fatal
	// we accumulate them and log.
	fkeys, err := RetrieveSSHPubKeysFromFiles()
	if err != nil {
		errs = errors.Join(err)
	}

	agentKeys, err := RetrieveSSHAgentKeys(ctx)
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
func RetrieveSSHAgentKeys(ctx context.Context) ([]ssh.PublicKey, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		log.Debug("SSH_AUTH_SOCK not set, skipping pubkey fetching")
		return nil, nil
	}

	dialer := net.Dialer{}

	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		if !isSSHAgentUnavailableErr(err) {
			return nil, fmt.Errorf("failed to open SSH_AUTH_SOCK: %w", err)
		}

		// Socket is inaccessible — typically because the process runs with
		// elevated privileges (setuid or sudo) while the socket is owned by
		// the invoking user. Retry the dial as the real user.
		conn, err = dialSSHAgentAsRealUser(ctx, socket)
		if err != nil {
			log.Debugf(
				"unable to connect to SSH_AUTH_SOCK %q, skipping agent pubkey fetching: %v",
				socket,
				err,
			)
			return nil, nil
		}
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	keys, err := agentClient.List()
	if err != nil {
		return nil, fmt.Errorf("error listing agent's pub keys %w", err)
	}

	log.Debugf("extracted %d keys from ssh-agent", len(keys))

	pubKeys := make([]ssh.PublicKey, len(keys))

	for idx, key := range keys {
		pkey, err := ssh.ParsePublicKey(key.Blob)
		if err != nil {
			return nil, err
		}

		pubKeys[idx] = pkey
	}

	return pubKeys, nil
}

func isSSHAgentUnavailableErr(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.ECONNREFUSED)
}

// dialSSHAgentAsRealUser dials the SSH agent socket after temporarily dropping
// the effective UID to the real (non-root) user. This handles the common case
// where the binary runs with elevated privileges (setuid or sudo) but the
// agent socket is owned by the invoking user.
func dialSSHAgentAsRealUser(ctx context.Context, socket string) (net.Conn, error) {
	uid := realUserID()
	if uid < 0 {
		return nil, fmt.Errorf("cannot determine original user to access SSH agent socket")
	}

	origEuid := os.Geteuid()

	if err := syscall.Seteuid(uid); err != nil {
		return nil, fmt.Errorf("failed to drop privileges for SSH agent dial: %w", err)
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", socket)

	if restoreErr := syscall.Seteuid(origEuid); restoreErr != nil {
		if conn != nil {
			conn.Close()
		}

		return nil, fmt.Errorf("failed to restore privileges after SSH agent dial: %w", restoreErr)
	}

	return conn, err
}

// realUserID returns the UID of the real (non-root) user when the process
// runs with elevated privileges. It handles two cases:
//   - sudo: SUDO_UID env var identifies the original user.
//   - setuid binary: real uid differs from effective uid (0).
//
// Returns -1 when we are already running as a regular user or when the
// original user cannot be determined.
func realUserID() int {
	if os.Geteuid() != 0 {
		return -1
	}

	if uidStr, ok := os.LookupEnv("SUDO_UID"); ok {
		uid, err := strconv.Atoi(uidStr)
		if err == nil && uid != 0 {
			return uid
		}
	}

	if realUID := syscall.Getuid(); realUID != 0 {
		return realUID
	}

	return -1
}

// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

const (
	clabAuthzKeysFName = "authorized_keys"
	pubKeysGlob        = "~/.ssh/*.pub"
	// authorized keys file path on a clab host that is used to create the clabAuthzKeys file
	authzKeysFPath = "~/.ssh/authorized_keys"
)

// CreateAuthzKeysFile creats the authorized_keys file in the lab directory
// if any files ~/.ssh/*.pub found
func (c *CLab) CreateAuthzKeysFile() error {
	b := new(bytes.Buffer)

	p, err := resolvePath(pubKeysGlob)
	if err != nil {
		return fmt.Errorf("failed resolving path %s", pubKeysGlob)
	}

	all, err := filepath.Glob(p)
	if err != nil {
		return fmt.Errorf("failed globbing the path %s", p)
	}

	f, err := resolvePath(authzKeysFPath)
	if err != nil {
		return fmt.Errorf("failed resolving path %s", authzKeysFPath)
	}

	if utils.FileExists(f) {
		log.Debugf("%s found, adding the public keys it contains", f)
		all = append(all, f)
	}

	if len(all) == 0 {
		log.Debug("no public keys found")
		return nil
	}

	log.Debugf("found public key files %q", all)

	for _, fn := range all {
		rb, _ := os.ReadFile(fn)
		b.Write(rb)
	}

	clabAuthzKeysFPath := filepath.Join(c.Dir.Lab, clabAuthzKeysFName)
	if err := utils.CreateFile(clabAuthzKeysFPath, b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(clabAuthzKeysFPath, 0644) // skipcq: GSC-G302
}

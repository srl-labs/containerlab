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
	authzFName  = "authorized_keys"
	pubKeysGlob = "~/.ssh/*.pub"
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
	log.Debugf("found public key files %q", all)

	for _, fn := range all {
		rb, _ := os.ReadFile(fn)
		b.Write(rb)
	}

	if err := utils.CreateFile(filepath.Join(c.Dir.Lab, authzFName), b.String()); err != nil {
		return err
	}

	// ensure authz_keys will have the permissions allowing it to be read by anyone
	return os.Chmod(p, 0644)
}

// Copyright 2022 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// InstallIPTablesFwdRule calls iptables to install `allow` rule for traffic destined nodes on the clab management network
func (c *CLab) InstallIPTablesFwdRule() (err error) {
	mgmtNet := c.GlobalRuntime().Mgmt()
	if mgmtNet.Bridge == "" {
		log.Debug("skipping setup of iptables forwarding rules for non-bridged management network")
		return
	}

	// first check if a rule already exists to not create duplicates
	v4checkCmd := "iptables -vL DOCKER-USER"

	res, err := exec.Command("sudo", strings.Split(v4checkCmd, " ")...).Output()
	if bytes.Contains(res, []byte(mgmtNet.Bridge)) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", mgmtNet.Bridge)
		return err
	}

	if err != nil {
		return err
	}

	v4cmd := fmt.Sprintf("iptables -I DOCKER-USER -o %s -j ACCEPT", mgmtNet.Bridge)

	_, err = exec.Command("sudo", strings.Split(v4cmd, " ")...).Output()
	if err != nil {
		return
	}

	return err
}

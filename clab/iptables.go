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
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/utils"
)

// InstallIPTablesFwdRule calls iptables to install `allow` rule for traffic destined nodes on the clab management network
func (c *CLab) InstallIPTablesFwdRule() (err error) {
	if c.GlobalRuntime().GetName() != runtime.DockerRuntime {
		log.Debug("iptables forwarding rules management is not supported for this runtime")
		return nil
	}

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

// DeleteIPTablesFwdRule deletes `allow` rule installed with InstallIPTablesFwdRule when the bridge interface doesn't exist anymore
func (c *CLab) DeleteIPTablesFwdRule(br string) (err error) {
	if c.GlobalRuntime().GetName() != runtime.DockerRuntime {
		log.Debug("iptables forwarding rules management is not supported for this runtime")
		return nil
	}

	if br == "" || br == "docker0" {
		log.Warn("wat1")
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return
	}

	_, err = utils.BridgeByName(br)
	if err == nil {
		log.Warn("here1")
		// nil error means the bridge interface was found, hence we don't need to delete the fwd rule, as the bridge is still in use
		return nil
	}

	v4cmd := fmt.Sprintf("iptables -D DOCKER-USER -o %s -j ACCEPT", br)

	_, err = exec.Command("sudo", strings.Split(v4cmd, " ")...).Output()
	if err != nil {
		log.Warn(err)
		return
	}

	return err
}

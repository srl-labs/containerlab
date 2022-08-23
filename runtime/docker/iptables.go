package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

const (
	iptCheckCmd = "-vL DOCKER-USER"
	iptAllowCmd = "-I DOCKER-USER -o %s -j ACCEPT -m comment --comment \"set by containerlab\""
	iptDelCmd   = "-D DOCKER-USER -o %s -j ACCEPT -m comment --comment \"set by containerlab\""
)

// installIPTablesFwdRule calls iptables to install `allow` rule for traffic destined nodes on the clab management network.
func (d *DockerRuntime) installIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of iptables forwarding rules for non-bridged management network")
		return
	}

	// first check if a rule already exists to not create duplicates
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()
	if bytes.Contains(res, []byte(d.mgmt.Bridge)) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", d.mgmt.Bridge)
		return err
	}
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("missing DOCKER-USER iptables chain. See http://containerlab.dev/manual/network/#external-access")
	}

	cmd, err := shlex.Split(fmt.Sprintf(iptAllowCmd, d.mgmt.Bridge))
	if err != nil {
		return err
	}

	log.Debugf("Installing iptables rules for bridge %q", d.mgmt.Bridge)

	stdOutErr, err := exec.Command("iptables", cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to install iptables rule using '%s' command: %w", cmd, err)
	}
	return nil
}

// deleteIPTablesFwdRule deletes `allow` rule installed with InstallIPTablesFwdRule when the bridge interface doesn't exist anymore.
func (d *DockerRuntime) deleteIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	br := d.mgmt.Bridge

	if br == "" || br == "docker0" {
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return
	}

	// first check if a rule exists before trying to delete it
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("missing DOCKER-USER iptables chain. See http://containerlab.dev/manual/network/#external-access")
	}

	if !bytes.Contains(res, []byte(br)) {
		log.Debug("external access iptables rule doesn't exist. Skipping deletion")
		return nil
	}

	// we are not deleting the rule if the bridge still exists
	// it happens when bridge is either still in use by docker network
	// or it is managed externally (created manually)
	_, err = utils.BridgeByName(br)
	if err == nil {
		log.Debugf("bridge %s is still in use, not removing the forwarding rule", br)
		return nil
	}

	cmd, err := shlex.Split(fmt.Sprintf(iptDelCmd, br))
	if err != nil {
		return err
	}

	log.Debugf("removing clab iptables rules for bridge %q", br)
	log.Debugf("trying to delete the forwarding rule with cmd: iptables %s", cmd)

	stdOutErr, err := exec.Command("iptables", cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables delete stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to delete iptables rules: %w", err)
	}

	return nil
}

package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

// installIPTablesFwdRule calls iptables to install `allow` rule for traffic destined nodes on the clab management network
func (d *DockerRuntime) installIPTablesFwdRule() (err error) {
	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of iptables forwarding rules for non-bridged management network")
		return
	}

	// first check if a rule already exists to not create duplicates
	v4checkCmd := "iptables -vL DOCKER-USER"

	res, err := exec.Command("sudo", strings.Split(v4checkCmd, " ")...).Output()
	if bytes.Contains(res, []byte(d.mgmt.Bridge)) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", d.mgmt.Bridge)
		return err
	}

	if err != nil {
		return err
	}

	v4cmd := fmt.Sprintf("iptables -I DOCKER-USER -o %s -j ACCEPT", d.mgmt.Bridge)

	_, err = exec.Command("sudo", strings.Split(v4cmd, " ")...).Output()
	if err != nil {
		return
	}

	return err
}

// deleteIPTablesFwdRule deletes `allow` rule installed with InstallIPTablesFwdRule when the bridge interface doesn't exist anymore
func (d *DockerRuntime) deleteIPTablesFwdRule(br string) (err error) {
	if br == "" || br == "docker0" {
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return
	}

	_, err = utils.BridgeByName(br)
	if err == nil {
		// nil error means the bridge interface was found, hence we don't need to delete the fwd rule, as the bridge is still in use
		return nil
	}

	v4cmd := fmt.Sprintf("iptables -D DOCKER-USER -o %s -j ACCEPT", br)

	_, err = exec.Command("sudo", strings.Split(v4cmd, " ")...).Output()
	if err != nil {
		return
	}

	return err
}

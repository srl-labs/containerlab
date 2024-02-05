package docker

import (
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime/docker/firewall"
)

// deleteFwdRule deletes `allow` rule installed with installFwdRule when the bridge interface doesn't exist anymore.
func (d *DockerRuntime) deleteFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	f, err := firewall.NewFirewallClient(d.mgmt.Bridge)
	if err != nil {
		return err
	}

	return f.DeleteForwardingRules()
}

// installFwdRule installs the `allow` rule for traffic destined to the nodes
// on the clab management network.
// This rule is required for external access to the nodes.
func (d *DockerRuntime) installFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of forwarding rules for non-bridged management network")
		return
	}

	f, err := firewall.NewFirewallClient(d.mgmt.Bridge)
	if err != nil {
		return err
	}
	log.Debugf("using %s as the firewall interface", f.Name())

	return f.InstallForwardingRules()
}

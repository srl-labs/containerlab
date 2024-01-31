package docker

import (
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime/docker/firewall"
)

// deleteIPTablesFwdRule deletes `allow` rule installed with InstallIPTablesFwdRule when the bridge interface doesn't exist anymore.
func (d *DockerRuntime) deleteIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	f, err := firewall.GetFirewallImplementation(d.mgmt.Bridge)
	if err != nil {
		return err
	}
	return f.DeleteForwardingRules()
}

// installIPTablesFwdRule installs the `allow` rule for traffic destined to the nodes
// on the clab management network.
// This rule is required for external access to the nodes.
func (d *DockerRuntime) installIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of forwarding rules for non-bridged management network")
		return
	}

	f, err := firewall.GetFirewallImplementation(d.mgmt.Bridge)
	if err != nil {
		return err
	}
	log.Debugf("using %s as the firewall interface", f.Name())

	return f.InstallForwardingRules()
}

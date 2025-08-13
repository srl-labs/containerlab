package docker

import (
	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/runtime/docker/firewall"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
)

// deleteMgmtNetworkFwdRule deletes `allow` rule installed with installFwdRule
// when the containerlab management network (bridge) interface doesn't exist anymore.
func (d *DockerRuntime) deleteMgmtNetworkFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return nil
	}

	f, err := firewall.NewFirewallClient()
	if err != nil {
		return err
	}

	// delete the rules with the management bridge listed as the outgoing interface with the allow action
	// in the DOCKER-USER chain
	r := definitions.FirewallRule{
		Interface: d.mgmt.Bridge,
		Direction: definitions.OutDirection,
		Chain:     definitions.DockerUserChain,
		Table:     definitions.FilterTable,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
	}

	err = f.DeleteForwardingRules(&r)
	if err != nil {
		return err
	}

	// install the rules with the management bridge listed as the incoming interface with the allow action
	// in the DOCKER-USER chain
	r = definitions.FirewallRule{
		Interface: d.mgmt.Bridge,
		Direction: definitions.OutDirection,
		Chain:     definitions.DockerUserChain,
		Table:     definitions.FilterTable,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
	}

	err = f.DeleteForwardingRules(&r)
	if err != nil {
		return err
	}

	return err
}

// installMgmtNetworkFwdRule installs the `allow` rule for traffic destined to the nodes
// on the clab management network for v4 and v6.
// This rule is required for external access to the nodes.
func (d *DockerRuntime) installMgmtNetworkFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		log.Debug("skipping setup of forwarding rules for the management network since External Access is disabled by a user")
		return nil
	}

	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of forwarding rules for non-bridged management network")
		return nil
	}

	f, err := firewall.NewFirewallClient()
	if err != nil {
		return err
	}
	log.Debugf("using %s as the firewall interface", f.Name())

	// install the rules with the management bridge listed as the outgoing interface with the allow action
	// in the DOCKER-USER chain
	r := definitions.FirewallRule{
		Interface: d.mgmt.Bridge,
		Direction: definitions.OutDirection,
		Chain:     definitions.DockerUserChain,
		Table:     definitions.FilterTable,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
	}

	err = f.InstallForwardingRules(&r)
	if err != nil {
		return err
	}

	// install the rules with the management bridge listed as the incoming interface with the allow action
	// in the DOCKER-USER chain
	r = definitions.FirewallRule{
		Interface: d.mgmt.Bridge,
		Direction: definitions.InDirection,
		Chain:     definitions.DockerUserChain,
		Table:     definitions.FilterTable,
		Action:    definitions.AcceptAction,
		Comment:   definitions.ContainerlabComment,
	}

	err = f.InstallForwardingRules(&r)
	if err != nil {
		return err
	}

	return err
}

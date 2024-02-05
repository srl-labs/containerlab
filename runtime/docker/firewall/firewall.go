package firewall

import (
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/iptables"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/nftables"
)

// NewFirewallClient returns a firewall client based on the availability of nftables or iptables.
func NewFirewallClient(bridgeName string) (definitions.ClabFirewall, error) {
	var clf definitions.ClabFirewall

	clf, err := nftables.NewNftablesClient(bridgeName)
	if err == nil {
		return clf, nil
	}

	clf, err = iptables.NewIpTablesClient(bridgeName)
	if err == nil {
		return clf, nil
	}

	return nil, err
}

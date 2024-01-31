package firewall

import (
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/iptables"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/nftables"
)

func GetFirewallImplementation(bridgeName string) (definitions.ClabFirewall, error) {
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

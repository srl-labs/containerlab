package iptables

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	"github.com/srl-labs/containerlab/utils"
)

const (
	iptCheckArgs = "-vL DOCKER-USER"
	iptAllowArgs = "-I DOCKER-USER -o %s -j ACCEPT -m comment --comment \"" + definitions.IPTablesRuleComment + "\""
	iptDelArgs   = "-D DOCKER-USER -o %s -j ACCEPT -m comment --comment \"" + definitions.IPTablesRuleComment + "\""
	ipTables     = "ip_tables"

	v4AF         = "v4"
	ip4tablesCmd = "iptables"
	v6AF         = "v6"
	ip6tablesCmd = "ip6tables"
)

// IpTablesClient is a client for iptables.
type IpTablesClient struct {
	bridgeName string
	ip6_tables bool
}

// NewIpTablesClient returns a new IpTablesClient.
func NewIpTablesClient(bridgeName string) (*IpTablesClient, error) {
	v4ModLoaded, err := utils.IsKernelModuleLoaded("ip_tables")
	if err != nil {
		return nil, err
	}

	v6ModLoaded, _ := utils.IsKernelModuleLoaded("ip6_tables")

	if !v4ModLoaded {
		log.Debug("ip_tables kernel module not available")
		// module is not loaded
		return nil, definitions.ErrNotAvailable
	}

	return &IpTablesClient{
		bridgeName: bridgeName,
		ip6_tables: v6ModLoaded,
	}, nil
}

// Name returns the name of the firewall client.
func (*IpTablesClient) Name() string {
	return ipTables
}

// InstallForwardingRules installs the forwarding rules for v4 and v6 address families.
func (c *IpTablesClient) InstallForwardingRules() error {
	err := c.InstallForwardingRulesForAF(v4AF)
	if err != nil {
		return err
	}

	if c.ip6_tables {
		err = c.InstallForwardingRulesForAF(v6AF)
	}

	return err
}

// InstallForwardingRulesForAF installs the forwarding rules for the specified address family.
func (c *IpTablesClient) InstallForwardingRulesForAF(af string) error {
	iptCmd := ip4tablesCmd
	if af == v6AF {
		iptCmd = ip6tablesCmd
	}

	// first check if a rule already exists to not create duplicates
	if c.allowRuleForMgmtBrExists(af) {
		return nil
	}

	cmd, err := shlex.Split(fmt.Sprintf(iptAllowArgs, c.bridgeName))
	if err != nil {
		return err
	}

	log.Debugf("Installing iptables (%s) rules for bridge %q", af, c.bridgeName)

	stdOutErr, err := exec.Command(iptCmd, cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to install iptables rule using '%s' command: %w", cmd, err)
	}

	return nil
}

// DeleteForwardingRules deletes the forwarding rules for v4 and v6 address families.
func (c *IpTablesClient) DeleteForwardingRules() error {
	err := c.DeleteForwardingRulesForAF(v4AF)
	if err != nil {
		return err
	}

	if c.ip6_tables {
		err = c.InstallForwardingRulesForAF(v6AF)
	}

	return err
}

// DeleteForwardingRulesForAF deletes the forwarding rules for a specified AF.
func (c *IpTablesClient) DeleteForwardingRulesForAF(af string) error {
	iptCmd := ip4tablesCmd
	if af == v6AF {
		iptCmd = ip6tablesCmd
	}

	// first check if a rule exists before trying to delete it
	res, err := exec.Command(iptCmd, strings.Split(iptCheckArgs, " ")...).Output()
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("missing DOCKER-USER iptables chain. See http://containerlab.dev/manual/network/#external-access")
	}

	if !bytes.Contains(res, []byte(c.bridgeName)) {
		log.Debug("external access iptables rule doesn't exist. Skipping deletion")
		return nil
	}

	// we are not deleting the rule if the bridge still exists
	// it happens when bridge is either still in use by docker network
	// or it is managed externally (created manually)
	_, err = utils.BridgeByName(c.bridgeName)
	if err == nil {
		log.Debugf("bridge %s is still in use, not removing the forwarding rule", c.bridgeName)
		return nil
	}

	cmd, err := shlex.Split(fmt.Sprintf(iptDelArgs, c.bridgeName))
	if err != nil {
		return err
	}

	log.Debugf("removing clab iptables rules for bridge %q", c.bridgeName)
	log.Debugf("trying to delete the forwarding rule with cmd: iptables %s", cmd)

	stdOutErr, err := exec.Command(iptCmd, cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables delete stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to delete iptables rules: %w", err)
	}

	return nil
}

// allowRuleForMgmtBrExists checks if an allow rule for the provided bridge name exists.
// The actual check doesn't verify that `allow` is set, it just checks if the rule
// has the provided bridge name in the output interface.
func (c *IpTablesClient) allowRuleForMgmtBrExists(af string) bool {
	iptCmd := ip4tablesCmd
	if af == v6AF {
		iptCmd = ip6tablesCmd
	}

	res, err := exec.Command(iptCmd, strings.Split(iptCheckArgs, " ")...).CombinedOutput()
	if err != nil {
		log.Warnf("iptables check error: %s. Output: %s", err, string(res))
		// if we errored on check we don't want to try setting up the rule
		return true
	}
	if bytes.Contains(res, []byte(c.bridgeName)) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", c.bridgeName)

		return true
	}

	return false
}

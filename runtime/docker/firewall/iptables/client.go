package iptables

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/google/shlex"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	clabutils "github.com/srl-labs/containerlab/utils"
)

const (
	iptCheckArgs = "-vL DOCKER-USER -w 5"
	iptAllowArgs = "-I DOCKER-USER -%s %s -j ACCEPT -w 5 -m comment --comment \"" + definitions.ContainerlabComment + "\""
	iptDelArgs   = "-D DOCKER-USER -%s %s -j ACCEPT -w 5 -m comment --comment \"" + definitions.ContainerlabComment + "\""
	ipTables     = "ip_tables"

	v4AF         = "v4"
	ip4tablesCmd = "iptables"
	v6AF         = "v6"
	ip6tablesCmd = "ip6tables"
)

var iptablesCmd = map[string]string{
	v4AF: ip4tablesCmd,
	v6AF: ip6tablesCmd,
}

// IpTablesClient is a client for iptables.
type IpTablesClient struct {
	ip6_tables bool
}

// NewIpTablesClient returns a new IpTablesClient.
func NewIpTablesClient() (*IpTablesClient, error) {
	v4ModLoaded, err := clabutils.IsKernelModuleLoaded("ip_tables")
	if err != nil {
		return nil, err
	}

	v6ModLoaded, _ := clabutils.IsKernelModuleLoaded("ip6_tables")

	if !v4ModLoaded {
		log.Debug("ip_tables kernel module not available")
		// module is not loaded
		return nil, definitions.ErrNotAvailable
	}

	return &IpTablesClient{
		ip6_tables: v6ModLoaded,
	}, nil
}

// Name returns the name of the firewall client.
func (*IpTablesClient) Name() string {
	return ipTables
}

// InstallForwardingRules installs the forwarding rules for v4 and v6 address families for the provided
// input or output interface and chain.
func (c *IpTablesClient) InstallForwardingRules(rule *definitions.FirewallRule) error {
	err := c.InstallForwardingRulesForAF(v4AF, rule)
	if err != nil {
		return err
	}

	if c.ip6_tables {
		err = c.InstallForwardingRulesForAF(v6AF, rule)
	}

	return err
}

// InstallForwardingRulesForAF installs the forwarding rules for the specified address family.
func (c *IpTablesClient) InstallForwardingRulesForAF(af string, rule *definitions.FirewallRule) error {
	iptCmd := ip4tablesCmd
	if af == v6AF {
		iptCmd = ip6tablesCmd
	}

	iface := rule.Interface

	// first check if a rule already exists to not create duplicates
	if c.ruleExists(af, rule) {
		return nil
	}

	directionFlag := strings.ToLower(string(rule.Direction[0]))

	cmd, err := shlex.Split(fmt.Sprintf(iptAllowArgs, directionFlag, iface))
	if err != nil {
		return err
	}

	log.Debugf("Installing iptables (%s) rules for bridge %q", af, iface)

	stdOutErr, err := exec.Command(iptCmd, cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to install iptables rule using '%s' command: %w", cmd, err)
	}

	return nil
}

// DeleteForwardingRules deletes the forwarding rules for v4 and v6 address families.
func (c *IpTablesClient) DeleteForwardingRules(rule *definitions.FirewallRule) error {
	err := c.DeleteForwardingRulesForAF(v4AF, rule)
	if err != nil {
		return err
	}

	if c.ip6_tables {
		err = c.DeleteForwardingRulesForAF(v6AF, rule)
	}

	return err
}

// DeleteForwardingRulesForAF deletes the forwarding rules for a specified AF.
func (c *IpTablesClient) DeleteForwardingRulesForAF(af string, rule *definitions.FirewallRule) error {
	iptCmd := ip4tablesCmd
	if af == v6AF {
		iptCmd = ip6tablesCmd
	}

	iface := rule.Interface

	// first check if a rule exists before trying to delete it
	res, err := exec.Command(iptCmd, strings.Split(iptCheckArgs, " ")...).Output()
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("missing DOCKER-USER iptables chain. See http://containerlab.dev/manual/network/#external-access")
	}

	if !bytes.Contains(res, []byte(iface)) {
		log.Debug("external access iptables rule doesn't exist. Skipping deletion")
		return nil
	}

	// we are not deleting the rule if the bridge still exists
	// it happens when bridge is either still in use by docker network
	// or it is managed externally (created manually)
	_, err = clabutils.BridgeByName(iface)
	if err == nil {
		log.Debugf("bridge %s is still in use, not removing the forwarding rule", iface)
		return nil
	}

	directionFlag := strings.ToLower(string(rule.Direction[0]))

	cmd, err := shlex.Split(fmt.Sprintf(iptDelArgs, directionFlag, iface))
	if err != nil {
		return err
	}

	log.Debugf("removing clab iptables rules for bridge %q", iface)
	log.Debugf("trying to delete the forwarding rule with cmd: iptables %s", cmd)

	stdOutErr, err := exec.Command(iptCmd, cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables delete stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to delete iptables rules: %w", err)
	}

	return nil
}

// ruleExists checks if an allow rule for the provided `rule` exists.
func (c *IpTablesClient) ruleExists(af string, rule *definitions.FirewallRule) bool {
	iptCmd := iptablesCmd[af]

	res, err := exec.Command(iptCmd, strings.Split(iptCheckArgs, " ")...).CombinedOutput()
	if err != nil {
		log.Warnf("iptables check error: %s. Output: %s", err, string(res))
		// if we errored on check we don't want to try setting up the rule
		return true
	}

	// replace contiguous whitespace chars from the result with a single space
	// to be able to compare the result with the iface and direction chars
	res = bytes.Join(bytes.Fields(res), []byte(" "))

	var matcher string
	if rule.Direction == definitions.InDirection {
		matcher = fmt.Sprintf("%s *", rule.Interface)
	}

	if rule.Direction == definitions.OutDirection {
		matcher = fmt.Sprintf("* %s", rule.Interface)
	}

	if bytes.Contains(res, []byte(matcher)) {
		log.Debugf("found iptables forwarding rule targeting the interface %q direction %s. Skipping creation of the forwarding rule", rule.Interface, rule.Direction)

		return true
	}

	return false
}

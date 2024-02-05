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
	iptCheckCmd = "-vL DOCKER-USER"
	iptAllowCmd = "-I DOCKER-USER -o %s -j ACCEPT -m comment --comment \"" + definitions.IPTablesRuleComment + "\""
	iptDelCmd   = "-D DOCKER-USER -o %s -j ACCEPT -m comment --comment \"" + definitions.IPTablesRuleComment + "\""
	ipTables    = "ip_tables"
)

// IpTablesClient is a client for iptables.
type IpTablesClient struct {
	bridgeName string
}

// NewIpTablesClient returns a new IpTablesClient.
func NewIpTablesClient(bridgeName string) (*IpTablesClient, error) {
	loaded, err := utils.IsKernelModuleLoaded("ip_tables")
	if err != nil {
		return nil, err
	}

	if !loaded {
		log.Debug("ip_tables kernel module not available")
		// module is not loaded
		return nil, definitions.ErrNotAvailabel
	}

	return &IpTablesClient{
		bridgeName: bridgeName,
	}, nil
}

// Name returns the name of the firewall client.
func (*IpTablesClient) Name() string {
	return ipTables
}

// InstallForwardingRules installs the forwarding rules.
func (c *IpTablesClient) InstallForwardingRules() error {
	// first check if a rule already exists to not create duplicates
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()
	if bytes.Contains(res, []byte(c.bridgeName)) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", c.bridgeName)
		return err
	}
	if err != nil {
		// non nil error typically means that DOCKER-USER chain doesn't exist
		// this happens with old docker installations (centos7 hello) from default repos
		return fmt.Errorf("missing DOCKER-USER iptables chain. See http://containerlab.dev/manual/network/#external-access")
	}

	cmd, err := shlex.Split(fmt.Sprintf(iptAllowCmd, c.bridgeName))
	if err != nil {
		return err
	}

	log.Debugf("Installing iptables rules for bridge %q", c.bridgeName)

	stdOutErr, err := exec.Command("iptables", cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables install stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to install iptables rule using '%s' command: %w", cmd, err)
	}

	return nil
}

// DeleteForwardingRules deletes the forwarding rules.
func (c *IpTablesClient) DeleteForwardingRules() error {
	// first check if a rule exists before trying to delete it
	res, err := exec.Command("iptables", strings.Split(iptCheckCmd, " ")...).Output()
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

	cmd, err := shlex.Split(fmt.Sprintf(iptDelCmd, c.bridgeName))
	if err != nil {
		return err
	}

	log.Debugf("removing clab iptables rules for bridge %q", c.bridgeName)
	log.Debugf("trying to delete the forwarding rule with cmd: iptables %s", cmd)

	stdOutErr, err := exec.Command("iptables", cmd...).CombinedOutput()
	if err != nil {
		log.Warnf("Iptables delete stdout/stderr result is: %s", stdOutErr)
		return fmt.Errorf("unable to delete iptables rules: %w", err)
	}

	return nil
}

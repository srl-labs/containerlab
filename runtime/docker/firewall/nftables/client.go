package nftables

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
	"github.com/srl-labs/containerlab/utils"
)

const nfTables = "nf_tables"

// NftablesClient is a client for nftables.
type NftablesClient struct {
	nftConn    *nftables.Conn
	bridgeName string
}

// NewNftablesClient returns a new NftablesClient.
func NewNftablesClient(bridgeName string) (*NftablesClient, error) {
	loaded, err := utils.IsKernelModuleLoaded("nf_tables")
	if err != nil {
		return nil, err
	}

	if !loaded {
		log.Debug("nf_tables kernel module not available")
		// module is not loaded
		return nil, definitions.ErrNotAvailabel
	}

	// setup netlink connection with nftables
	nftConn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, err
	}

	nftC := &NftablesClient{
		nftConn:    nftConn,
		bridgeName: bridgeName,
	}

	chains, err := nftC.getChains(definitions.DockerFWUserChain)
	if err != nil {
		return nil, err
	}
	if len(chains) == 0 {
		log.Debugf("nftables does not seem to be in use, no %s chain found.", definitions.DockerFWUserChain)
		return nil, definitions.ErrNotAvailabel
	}

	return nftC, nil
}

// Name returns the name of the firewall client.
func (*NftablesClient) Name() string {
	return nfTables
}

// DeleteForwardingRules deletes the forwarding rules.
func (c *NftablesClient) DeleteForwardingRules() error {
	if c.bridgeName == "docker0" {
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return nil
	}

	// first check if a rule already exists to not create duplicates
	defer c.close()

	rules, err := c.getRules(definitions.DockerFWUserChain, definitions.DockerFWTable, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	mgmtBrRules := c.getRulesForMgmtBr(c.bridgeName, rules)
	if len(mgmtBrRules) == 0 {
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

	log.Debugf("removing clab iptables rules for bridge %q", c.bridgeName)
	for _, r := range mgmtBrRules {
		c.deleteRule(r)
	}

	c.flush()
	return nil
}

// InstallForwardingRules installs the forwarding rules.
func (c *NftablesClient) InstallForwardingRules() error {
	defer c.close()

	rules, err := c.getRules(definitions.DockerFWUserChain, definitions.DockerFWTable, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	if c.allowRuleForMgmtBrExists(c.bridgeName, rules) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", c.bridgeName)
		return nil
	}

	log.Debugf("Installing iptables rules for bridge %q", c.bridgeName)

	// create a new rule
	rule, err := c.newClabNftablesRule(definitions.DockerFWUserChain, definitions.DockerFWTable, nftables.TableFamilyIPv4, 0)
	if err != nil {
		return err
	}
	// set Output interface match
	rule.AddOutputInterfaceFilter(c.bridgeName)

	// add a comment
	err = rule.AddComment(definitions.IPTablesRuleComment)
	if err != nil {
		return err
	}
	// add a counter
	err = rule.AddCounter()
	if err != nil {
		return err
	}
	// make it an ACCEPT rule
	err = rule.AddVerdictAccept()
	if err != nil {
		return err
	}
	// mark and note for installation
	c.insertRule(rule.rule)
	// flush changes out to nftables
	c.flush()

	return nil
}

func (nftC *NftablesClient) getChains(name string) ([]*nftables.Chain, error) {
	var result []*nftables.Chain

	chains, err := nftC.nftConn.ListChains()
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Name == name {
			result = append(result, c)
		}
	}
	return result, nil
}

func (nftC *NftablesClient) deleteRule(r *nftables.Rule) {
	nftC.nftConn.DelRule(r)
}

// getRules returns all rules for the provided chain name, table name and family.
func (nftC *NftablesClient) getRules(chainName, tableName string, family nftables.TableFamily) ([]*nftables.Rule, error) {
	// get chain reference
	chains, err := nftC.getChains(chainName)
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Table.Name == tableName && c.Table.Family == family {
			return nftC.nftConn.GetRules(c.Table, c)
		}
	}

	return nil, fmt.Errorf("no match for chain %q, table %q with family %q found", chainName, tableName, family)
}

func (nftC *NftablesClient) newClabNftablesRule(chainName, tableName string,
	family nftables.TableFamily, position uint64,
) (*clabNftablesRule, error) {
	chains, err := nftC.getChains(chainName)
	if err != nil {
		return nil, err
	}

	for _, c := range chains {
		if c.Table.Name == tableName && c.Table.Family == family {
			r := &nftables.Rule{
				Handle:   0,
				Position: position,
				Table:    c.Table,
				Chain:    c,
				Exprs:    []expr.Any{},
			}

			return &clabNftablesRule{rule: r}, nil
		}
	}

	return nil, errors.New("chain " + chainName + " not found in table " + tableName + " with family " + string(family))
}

// InsertRule inserts a rule.
func (nftC *NftablesClient) insertRule(r *nftables.Rule) {
	nftC.nftConn.InsertRule(r)
}

// Flush sends all buffered commands in a single batch to nftables.
func (nftC *NftablesClient) flush() error {
	return nftC.nftConn.Flush()
}

// Close closes connection to nftables.
func (nftC *NftablesClient) close() {
	nftC.nftConn.CloseLasting()
}

// allowRuleForMgmtBrExists checks if an allow rule for the provided bridge name exists.
// The actual check doesn't verify that `allow` is set, it just checks if the rule
// has the provided bridge name in the output interface match and has a comment that is setup
// by containerlab.
func (nftC *NftablesClient) allowRuleForMgmtBrExists(brName string, rules []*nftables.Rule) bool {
	return len(nftC.getRulesForMgmtBr(brName, rules)) > 0
}

// getRulesForMgmtBr returns all rules that have the provided bridge name in the output interface match
// and have a comment that is setup by containerlab.
func (*NftablesClient) getRulesForMgmtBr(brName string, rules []*nftables.Rule) []*nftables.Rule {
	var result []*nftables.Rule

	for _, r := range rules {
		oifNameFound := false
		commentFound := false

		for _, e := range r.Exprs {
			switch v := e.(type) {
			// Cmp is a comparison expression
			// in the case of the rule we are looking for, it should be a comparison of the output interface name
			case *expr.Cmp:
				if bytes.Equal(v.Data, []byte(brName+"\x00")) {
					oifNameFound = true
				}
			// Match is a match expression
			// in the case of the rule we are looking for, it should contain
			// a comment extension with the comment set by containerlab
			case *expr.Match:
				if v.Name == "comment" {
					if val, ok := v.Info.(*xt.Unknown); ok {
						if bytes.HasPrefix(*val, []byte(definitions.IPTablesRuleComment)) {
							commentFound = true
						}
					}
				}
			default:
				continue
			}
		}

		if oifNameFound && commentFound {
			result = append(result, r)
		}
	}

	return result
}

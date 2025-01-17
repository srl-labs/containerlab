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
	nftConn *nftables.Conn
	// is ip6_tables supported
	ip6_tables bool
}

// NewNftablesClient returns a new NftablesClient.
func NewNftablesClient() (*NftablesClient, error) {
	// setup netlink connection with nftables
	nftConn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, err
	}

	nftC := &NftablesClient{
		nftConn:    nftConn,
		ip6_tables: true,
	}

	chains, err := nftC.getChains(definitions.DockerUserChain, nftables.TableFamilyIPv4)
	if err != nil {
		return nil, err
	}
	if len(chains) == 0 {
		log.Debugf("nftables does not seem to be in use, no %s chain found.", definitions.DockerUserChain)
		return nil, definitions.ErrNotAvailable
	}

	// check if ip6_tables is available
	v6Tables, err := nftC.nftConn.ListTablesOfFamily(nftables.TableFamilyIPv6)
	if err != nil || len(v6Tables) == 0 {
		nftC.ip6_tables = false
	}

	return nftC, nil
}

// Name returns the name of the firewall client.
func (*NftablesClient) Name() string {
	return nfTables
}

// DeleteForwardingRules deletes the forwarding rules for in or out interface in a given chain.
func (c *NftablesClient) DeleteForwardingRules(rule definitions.FirewallRule) error {
	iface := rule.Interface
	if iface == "docker0" {
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return nil
	}

	defer c.close()

	allRules, err := c.getRules(rule.Chain, rule.Table, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	var v6rules []*nftables.Rule
	if c.ip6_tables {
		v6rules, err = c.getRules(rule.Chain, rule.Table, nftables.TableFamilyIPv4)
		if err != nil {
			return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
		}
	}

	allRules = append(allRules, v6rules...)

	clabRules := c.getClabRulesForInterface(iface, allRules)
	if len(clabRules) == 0 {
		log.Debug("external access iptables rule doesn't exist. Skipping deletion")
		return nil
	}

	// we are not deleting the rule if the bridge still exists
	// it happens when bridge is either still in use by docker network
	// or it is managed externally (created manually)
	_, err = utils.BridgeByName(rule.Interface)
	if err == nil {
		log.Debugf("bridge %s is still in use, not removing the forwarding rule", iface)
		return nil
	}

	log.Debugf("removing clab iptables rules for bridge %q", iface)
	for _, r := range clabRules {
		c.deleteRule(r)
	}

	c.flush()
	return nil
}

// InstallForwardingRules installs the forwarding rules for v4 and v6 address families for the provided
// input or output interface and chain.
func (c *NftablesClient) InstallForwardingRules(rule definitions.FirewallRule) error {
	defer c.close()

	err := c.InstallForwardingRulesForAF(nftables.TableFamilyIPv4, rule)
	if err != nil {
		return err
	}

	if c.ip6_tables {
		err = c.InstallForwardingRulesForAF(nftables.TableFamilyIPv6, rule)
	}

	return err
}

// InstallForwardingRulesForAF installs the forwarding rules for the specified address family
// input interface and chain.
func (c *NftablesClient) InstallForwardingRulesForAF(af nftables.TableFamily, rule definitions.FirewallRule) error {
	iface := rule.Interface

	rules, err := c.getRules(rule.Chain, rule.Table, af)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	if c.allowRuleExistsForInterface(iface, rules) {
		log.Debugf("found allowing iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", iface)
		return nil
	}

	log.Debugf("Installing iptables rules for bridge %q", iface)

	// create a new rule
	r, err := c.newClabNftablesRule(rule.Chain, rule.Table, af, 0)
	if err != nil {
		return err
	}
	// set interface match
	r.AddInterfaceFilter(rule)

	// add a comment
	err = r.AddComment(rule.Comment)
	if err != nil {
		return err
	}
	// add a counter
	err = r.AddCounter()
	if err != nil {
		return err
	}
	// make it an ACCEPT rule
	err = r.AddVerdictAccept()
	if err != nil {
		return err
	}
	// mark and note for installation
	c.insertRule(r.rule)
	// flush changes out to nftables
	c.flush()

	return nil
}

// getChains returns all chains with the provided name and family.
func (nftC *NftablesClient) getChains(name string, family nftables.TableFamily) ([]*nftables.Chain, error) {
	var result []*nftables.Chain

	chains, err := nftC.nftConn.ListChainsOfTableFamily(family)
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
	chains, err := nftC.getChains(chainName, family)
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
	chains, err := nftC.getChains(chainName, family)
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

// allowRuleExistsForInterface checks if an allow rule for the provided interface name exists.
// The actual check doesn't verify that `allow` is set, it just checks if the rule
// has the provided interface name and has a comment that is setup
// by containerlab.
func (nftC *NftablesClient) allowRuleExistsForInterface(iface string, rules []*nftables.Rule) bool {
	return len(nftC.getClabRulesForInterface(iface, rules)) > 0
}

// getClabRulesForInterface returns rules that have the provided interface name in the output interface match
// and have a comment that is setup by containerlab from the list of `rules`.
func (*NftablesClient) getClabRulesForInterface(iface string, rules []*nftables.Rule) []*nftables.Rule {
	var result []*nftables.Rule

	for _, r := range rules {
		oifNameFound := false
		commentFound := false

		for _, e := range r.Exprs {
			switch v := e.(type) {
			// Cmp is a comparison expression
			// in the case of the rule we are looking for, it should be a comparison of the output interface name
			case *expr.Cmp:
				if bytes.Equal(v.Data, []byte(iface+"\x00")) {
					oifNameFound = true
				}
			// Match is a match expression
			// in the case of the rule we are looking for, it should contain
			// a comment extension with the comment set by containerlab
			case *expr.Match:
				if v.Name == "comment" {
					if val, ok := v.Info.(*xt.Unknown); ok {
						if bytes.HasPrefix(*val, []byte(definitions.ContainerlabComment)) {
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

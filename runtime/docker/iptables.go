package docker

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

const (
	DockerFWUserChain = "DOCKER-USER"
	DockerFWTable     = "filter"

	IPTablesRuleComment = "set by containerlab"

	IPTablesCommentMaxSize = 256
)

type nftablesClient struct {
	nftConn *nftables.Conn
}

func newNftablesClient() (*nftablesClient, error) {
	// setup netlink connection with nftables
	nftConn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, err
	}

	nftC := &nftablesClient{
		nftConn: nftConn,
	}

	return nftC, nil
}

func (nftC *nftablesClient) GetChains(name string) ([]*nftables.Chain, error) {
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

func (nftC *nftablesClient) GetTable(family nftables.TableFamily, name string) (*nftables.Table, error) {
	tables, err := nftC.nftConn.ListTablesOfFamily(family)
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("table %q not found", name)
}

func (nftC *nftablesClient) DeleteRule(r *nftables.Rule) {
	nftC.nftConn.DelRule(r)
}

// GetRules returns all rules for the provided chain name, table name and family.
func (nftC *nftablesClient) GetRules(chainName, tableName string, family nftables.TableFamily) ([]*nftables.Rule, error) {
	// get chain reference
	chains, err := nftC.GetChains(chainName)
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

type clabNftablesRule struct {
	rule *nftables.Rule
}

func (nftC *nftablesClient) newClabNftablesRule(chainName, tableName string, family nftables.TableFamily, position uint64) (*clabNftablesRule, error) {
	chains, err := nftC.GetChains(chainName)
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

func (cnr *clabNftablesRule) AddOutputInterfaceFilter(oif string) {
	// define the metadata to evaluate
	metaOifName := &expr.Meta{
		Key:            expr.MetaKeyOIFNAME,
		SourceRegister: false,
		Register:       1,
	}
	// define the comparison
	comp := &expr.Cmp{
		Op:       expr.CmpOpEq,
		Register: 1,
		Data:     []byte(oif + "\x00"),
	}

	// add expr to rule
	cnr.rule.Exprs = append(cnr.rule.Exprs, metaOifName, comp)
}

func (cnr *clabNftablesRule) AddCounter() error {
	c := &expr.Counter{}
	cnr.rule.Exprs = append(cnr.rule.Exprs, c)
	return nil
}

func (cnr *clabNftablesRule) AddVerdictAccept() error {
	v := &expr.Verdict{
		Kind: expr.VerdictAccept,
	}
	cnr.rule.Exprs = append(cnr.rule.Exprs, v)
	return nil
}

func (cnr *clabNftablesRule) AddVerdictDrop() error {
	v := &expr.Verdict{
		Kind: expr.VerdictDrop,
	}
	cnr.rule.Exprs = append(cnr.rule.Exprs, v)
	return nil
}

// AddComment adds a comment to the rule.
func (cnr *clabNftablesRule) AddComment(comment string) error {
	// convert comment to byte
	actualCommentByte := []byte(comment)
	// check comment length not exceded
	if len(actualCommentByte) > IPTablesCommentMaxSize {
		return fmt.Errorf("comment max length is %d you've provided %d bytes", IPTablesCommentMaxSize, len(actualCommentByte))
	}

	// copy into byte alice of XT_MAX_COMMENT_LEN length
	commentBytes := make([]byte, IPTablesCommentMaxSize)
	copy(commentBytes, actualCommentByte)

	// create extension Info parameter as Unknown extension
	commentXTInfo := xt.Unknown(commentBytes)

	// create the extension
	e := &expr.Match{
		Name: "comment",
		Info: &commentXTInfo,
	}
	// add extension to rule
	cnr.rule.Exprs = append(cnr.rule.Exprs, e)
	return nil
}

// InsertRule inserts a rule.
func (nftC *nftablesClient) InsertRule(r *nftables.Rule) {
	nftC.nftConn.InsertRule(r)
}

// Flush sends all buffered commands in a single batch to nftables.
func (nftC *nftablesClient) Flush() error {
	return nftC.nftConn.Flush()
}

// Close closes connection to nftables.
func (nftC *nftablesClient) Close() {
	nftC.nftConn.CloseLasting()
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

	// first check if a rule already exists to not create duplicates
	nftC, err := newNftablesClient()
	if err != nil {
		return err
	}
	defer nftC.Close()

	rules, err := nftC.GetRules(DockerFWUserChain, DockerFWTable, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	if allowRuleForMgmtBrExists(d.mgmt.Bridge, rules) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", d.mgmt.Bridge)
		return nil
	}

	log.Debugf("Installing iptables rules for bridge %q", d.mgmt.Bridge)

	// create a new rule
	rule, err := nftC.newClabNftablesRule(DockerFWUserChain, DockerFWTable, nftables.TableFamilyIPv4, 0)
	if err != nil {
		return err
	}
	// set Output interface match
	rule.AddOutputInterfaceFilter(d.mgmt.Bridge)

	// add a comment
	err = rule.AddComment(IPTablesRuleComment)
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
	nftC.InsertRule(rule.rule)
	// flush changes out to nftables
	nftC.Flush()

	return nil
}

// allowRuleForMgmtBrExists checks if an allow rule for the provided bridge name exists.
// The actual check doesn't verify that `allow` is set, it just checks if the rule
// has the provided bridge name in the output interface match and has a comment that is setup
// by containerlab.
func allowRuleForMgmtBrExists(brName string, rules []*nftables.Rule) bool {
	return len(getRulesForMgmtBr(brName, rules)) > 0
}

// getRulesForMgmtBr returns all rules that have the provided bridge name in the output interface match
// and have a comment that is setup by containerlab.
func getRulesForMgmtBr(brName string, rules []*nftables.Rule) []*nftables.Rule {
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
						if bytes.HasPrefix(*val, []byte(IPTablesRuleComment)) {
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

// deleteIPTablesFwdRule deletes `allow` rule installed with InstallIPTablesFwdRule when the bridge interface doesn't exist anymore.
func (d *DockerRuntime) deleteIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	br := d.mgmt.Bridge

	if br == "docker0" {
		log.Debug("skipping deletion of iptables forwarding rule for non-bridged or default management network")
		return
	}

	// first check if a rule already exists to not create duplicates
	nftC, err := newNftablesClient()
	if err != nil {
		return err
	}
	defer nftC.Close()

	rules, err := nftC.GetRules(DockerFWUserChain, DockerFWTable, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	mgmtBrRules := getRulesForMgmtBr(d.mgmt.Bridge, rules)
	if len(mgmtBrRules) == 0 {
		log.Debug("external access iptables rule doesn't exist. Skipping deletion")
	}

	// we are not deleting the rule if the bridge still exists
	// it happens when bridge is either still in use by docker network
	// or it is managed externally (created manually)
	_, err = utils.BridgeByName(br)
	if err == nil {
		log.Debugf("bridge %s is still in use, not removing the forwarding rule", br)
		return nil
	}

	log.Debugf("removing clab iptables rules for bridge %q", br)
	for _, r := range mgmtBrRules {
		nftC.DeleteRule(r)
	}

	nftC.Flush()

	return nil
}

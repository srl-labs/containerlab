package docker

import (
	"bytes"
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
)

const (
	DOCKER_FW_USER_CHAIN = "DOCKER-USER"
	DOCKER_FW_TABLE      = "filter"

	COMMENT = "set by containerlab"

	XT_MAX_COMMENT_LEN = 256
)

type nftablesHelper struct {
	nftConn *nftables.Conn
}

func NewNftablesHelper() (*nftablesHelper, error) {
	// setup an nft connection
	nftConn, err := nftables.New(nftables.AsLasting())
	if err != nil {
		return nil, err
	}
	// build the nftablesHelper
	nfth := &nftablesHelper{
		nftConn: nftConn,
	}
	return nfth, nil
}

func (nfth *nftablesHelper) GetChains(name string) ([]*nftables.Chain, error) {
	chains, err := nfth.nftConn.ListChains()
	if err != nil {
		return nil, err
	}

	result := []*nftables.Chain{}

	for _, c := range chains {
		if c.Name == name {
			result = append(result, c)
		}
	}
	return result, nil
}

func (nfth *nftablesHelper) GetTable(family nftables.TableFamily, name string) (*nftables.Table, error) {
	tables, err := nfth.nftConn.ListTablesOfFamily(family)
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

func (nfth *nftablesHelper) DeleteRule(r *nftables.Rule) {
	nfth.nftConn.DelRule(r)
}

func (nfth *nftablesHelper) GetRules(chainName, tableName string, family nftables.TableFamily) ([]*nftables.Rule, error) {
	// get chain reference
	chains, err := nfth.GetChains(chainName)
	if err != nil {
		return nil, err
	}

	// // get rules
	// rules, err := nfth.nftConn.GetRules(c.Table, c)
	// if err != nil {
	// 	return nil, err
	// }

	for _, c := range chains {
		if c.Table.Name == tableName && c.Table.Family == family {
			return nfth.nftConn.GetRules(c.Table, c)
		}
	}

	return nil, fmt.Errorf("no match searching for chain %q under table %q with family %q", chainName, tableName, family)
}

type clabNftablesRule struct {
	rule *nftables.Rule
	nfth *nftablesHelper
}

func (nfth *nftablesHelper) newClabNftablesRule(chainName, tableName string, family nftables.TableFamily, position uint64) (*clabNftablesRule, error) {
	chains, err := nfth.GetChains(chainName)
	if err != nil {
		return nil, err
	}

	var chain *nftables.Chain

	for _, c := range chains {
		if c.Table.Name == tableName && c.Table.Family == family {
			chain = c
			break
		}
	}

	if chain == nil {
		return nil, fmt.Errorf("chain %q not found in table %q with family %q", chainName, tableName, family)
	}

	r := &nftables.Rule{
		Handle:   0,
		Position: position,
		Table:    chain.Table,
		Chain:    chain,
		Exprs:    []expr.Any{},
	}

	return &clabNftablesRule{rule: r, nfth: nfth}, nil
}

func (cnr *clabNftablesRule) AddOutputInterfaceFilter(oif string) error {
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
	return nil
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

func (cnr *clabNftablesRule) AddComment(comment string) error {
	// convert comment to byte
	actualCommentByte := []byte(comment)
	// check comment length not exceded
	if len(actualCommentByte) > XT_MAX_COMMENT_LEN {
		return fmt.Errorf("comment max length is %d you've provided %d bytes", XT_MAX_COMMENT_LEN, len(actualCommentByte))
	}

	// copy into byte alice of XT_MAX_COMMENT_LEN length
	commentBytes := make([]byte, XT_MAX_COMMENT_LEN)
	copy(commentBytes, []byte(comment))

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

func (cnr *clabNftablesRule) Install() {
	cnr.nfth.nftConn.InsertRule(cnr.rule)
}

// Flush sends all buffered commands in a single batch to nftables.
func (nfth *nftablesHelper) Flush() error {
	return nfth.nftConn.Flush()
}

// Close
func (nfth *nftablesHelper) Close() {
	nfth.nftConn.CloseLasting()
}

// installIPTablesFwdRule calls iptables to install `allow` rule for traffic destined nodes on the clab management network.
func (d *DockerRuntime) installIPTablesFwdRule() (err error) {
	if !*d.mgmt.ExternalAccess {
		return
	}

	if d.mgmt.Bridge == "" {
		log.Debug("skipping setup of forwarding rules for non-bridged management network")
		return
	}

	// first check if a rule already exists to not create duplicates
	nfth, err := NewNftablesHelper()
	if err != nil {
		return err
	}
	defer nfth.Close()

	rules, err := nfth.GetRules(DOCKER_FW_USER_CHAIN, DOCKER_FW_TABLE, nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("%w. See http://containerlab.dev/manual/network/#external-access", err)
	}

	if ruleForMgmtBrExists(d.mgmt.Bridge, rules) {
		log.Debugf("found iptables forwarding rule targeting the bridge %q. Skipping creation of the forwarding rule.", d.mgmt.Bridge)
		return nil
	}

	log.Debugf("Installing iptables rules for bridge %q", d.mgmt.Bridge)

	// create a new rule
	clnftr, err := nfth.newClabNftablesRule(DOCKER_FW_USER_CHAIN, DOCKER_FW_TABLE, nftables.TableFamilyIPv4, 0)
	if err != nil {
		return err
	}
	// set Output interface match
	err = clnftr.AddOutputInterfaceFilter(d.mgmt.Bridge)
	if err != nil {
		return err
	}
	// add a comment
	err = clnftr.AddComment(COMMENT)
	if err != nil {
		return err
	}
	// add a counter
	err = clnftr.AddCounter()
	if err != nil {
		return err
	}
	// make it an ACCEPT rule
	err = clnftr.AddVerdictAccept()
	if err != nil {
		return err
	}
	// mark and note for installation
	clnftr.Install()
	// flush changes out to nftables
	nfth.Flush()

	return nil
}

// ruleForMgmtBrExists checks if in the provided rules, there is a
// rule, that contains the provided brName
func ruleForMgmtBrExists(brName string, rules []*nftables.Rule) bool {
	return len(getRulesForMgmtBr(brName, rules)) > 0
}

func getRulesForMgmtBr(brName string, rules []*nftables.Rule) []*nftables.Rule {
	// contains all Matching rules
	result := []*nftables.Rule{}
	// iterate through rules
	for _, r := range rules {
		oifNameFound := false
		commentFound := false
		// iterate through rule expressions
		for _, e := range r.Exprs {
			switch e := e.(type) {
			case *expr.Cmp:
				// check if it contains the mgmt bridge name
				if bytes.Equal(e.Data, []byte(brName+"\x00")) {
					oifNameFound = true
				}
			case *expr.Match:
				if e.Name == "comment" {
					if val, ok := e.Info.(*xt.Unknown); ok {
						if bytes.HasPrefix(*val, []byte(COMMENT)) {
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
	nfth, err := NewNftablesHelper()
	if err != nil {
		return err
	}
	defer nfth.Close()

	rules, err := nfth.GetRules(DOCKER_FW_USER_CHAIN, DOCKER_FW_TABLE, nftables.TableFamilyIPv4)
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
		nfth.DeleteRule(r)
	}

	nfth.Flush()

	return nil
}

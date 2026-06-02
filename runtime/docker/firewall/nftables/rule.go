package nftables

import (
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	"github.com/srl-labs/containerlab/runtime/docker/firewall/definitions"
)

type clabNftablesRule struct {
	rule *nftables.Rule
}

// AddInterfaceFilter adds an interface filter to the rule.
func (cnr *clabNftablesRule) AddInterfaceFilter(rule *definitions.FirewallRule) {
	// define the metadata to evaluate
	metaKey := directionMap[rule.Direction]

	meta := &expr.Meta{
		Key:            metaKey,
		SourceRegister: false,
		Register:       1,
	}

	// define the comparison
	comp := &expr.Cmp{
		Op:       expr.CmpOpEq,
		Register: 1,
		Data:     []byte(rule.Interface + "\x00"),
	}

	// add expr to rule
	cnr.rule.Exprs = append(cnr.rule.Exprs, meta, comp)
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
	// check comment length
	if len(actualCommentByte) > definitions.IPTablesCommentMaxSize {
		return fmt.Errorf("comment max length is %d you've provided %d bytes",
			definitions.IPTablesCommentMaxSize, len(actualCommentByte))
	}

	// copy into byte alice of XT_MAX_COMMENT_LEN length
	commentBytes := make([]byte, definitions.IPTablesCommentMaxSize)
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

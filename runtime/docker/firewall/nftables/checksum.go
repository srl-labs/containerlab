package nftables

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	"golang.org/x/sys/unix"
)

const (
	checksumTable = "containerlab"
	checksumChain = "csum"
)

// SetTCPChecksumFill installs or removes a postrouting CHECKSUM fill rule on iface.
func SetTCPChecksumFill(iface string, enable bool) error {
	if iface == "" || strings.ContainsAny(iface, " \t\n") {
		return fmt.Errorf("invalid interface name %q", iface)
	}
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	var errs error
	ok := false
	for _, family := range []nftables.TableFamily{
		nftables.TableFamilyIPv4, nftables.TableFamilyIPv6,
	} {
		if err := checksumFillFamily(conn, family, iface, enable); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		ok = true
	}
	if ok {
		return nil
	}
	return errs
}

var checksumFillFamily = setTCPChecksumFill

func setTCPChecksumFill(
	conn *nftables.Conn,
	family nftables.TableFamily,
	iface string,
	enable bool,
) error {
	table := &nftables.Table{Family: family, Name: checksumTable}
	chain := &nftables.Chain{
		Table:    table,
		Name:     checksumChain,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityMangle,
	}
	if enable {
		conn.AddTable(table)
		conn.AddChain(chain)
		if err := conn.Flush(); err != nil {
			return err
		}
	}
	rules, err := conn.GetRules(table, chain)
	if err != nil {
		if !enable {
			return nil
		}
		return err
	}
	existing := checksumFillRules(rules, iface)
	if enable {
		if len(existing) > 0 {
			return nil
		}
		conn.AddRule(&nftables.Rule{Table: table, Chain: chain, Exprs: checksumFillExprs(iface)})
		return conn.Flush()
	}
	if len(existing) == 0 {
		return nil
	}
	for _, rule := range existing {
		conn.DelRule(rule)
	}
	return conn.Flush()
}

func checksumFillExprs(iface string) []expr.Any {
	info := xt.Unknown{0x01, 0, 0, 0, 0, 0, 0, 0} // XT_CHECKSUM_OP_FILL
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte(iface + "\x00")},
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
		&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_TCP}},
		&expr.Target{Name: "CHECKSUM", Info: &info},
	}
}

func checksumFillRules(rules []*nftables.Rule, iface string) []*nftables.Rule {
	want := iface + "\x00"
	var match []*nftables.Rule
	for _, rule := range rules {
		name, checksum := false, false
		for _, e := range rule.Exprs {
			switch v := e.(type) {
			case *expr.Cmp:
				if string(v.Data) == want {
					name = true
				}
			case *expr.Target:
				checksum = v.Name == "CHECKSUM"
			}
		}
		if name && checksum {
			match = append(match, rule)
		}
	}
	return match
}

package nftables

import (
	"fmt"
	"strings"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/google/nftables/xt"
	"golang.org/x/sys/unix"
)

const (
	checksumTable = "mangle"
	checksumChain = "csum"
)

// SetTCPChecksumFill installs or removes a postrouting CHECKSUM fill rule on iface.
func SetTCPChecksumFill(iface string, family int, enable bool) error {
	if iface == "" || strings.ContainsAny(iface, " \t\n") {
		return fmt.Errorf("invalid interface name %q", iface)
	}
	var tableFamily nftables.TableFamily
	switch family {
	case unix.AF_INET:
		tableFamily = nftables.TableFamilyIPv4
	case unix.AF_INET6:
		tableFamily = nftables.TableFamilyIPv6
	default:
		return fmt.Errorf("unsupported address family %d", family)
	}
	conn, err := nftables.New()
	if err != nil {
		return err
	}
	return checksumFillFamily(conn, tableFamily, iface, enable)
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

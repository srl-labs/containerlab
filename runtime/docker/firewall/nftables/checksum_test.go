package nftables

import (
	"errors"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func TestChecksumFillExprs(t *testing.T) {
	exprs := checksumFillExprs("cm-test")
	if len(checksumFillRules([]*nftables.Rule{{Exprs: exprs}}, "cm-test")) != 1 {
		t.Fatal("own exprs did not match")
	}
	if len(checksumFillRules([]*nftables.Rule{{Exprs: exprs}}, "cm-other")) != 0 {
		t.Fatal("matched a different interface")
	}
	if _, ok := exprs[len(exprs)-1].(*expr.Target); !ok {
		t.Fatal("missing CHECKSUM target")
	}
}

func TestSetTCPChecksumFillSingleStack(t *testing.T) {
	orig := checksumFillFamily
	t.Cleanup(func() { checksumFillFamily = orig })

	errV4 := errors.New("ipv4 unavailable")
	errV6 := errors.New("ipv6 unavailable")

	for _, tc := range []struct {
		name    string
		fail    map[nftables.TableFamily]error
		wantErr bool
	}{
		{"dual stack", nil, false},
		{"v4 only", map[nftables.TableFamily]error{nftables.TableFamilyIPv6: errV6}, false},
		{"v6 only", map[nftables.TableFamily]error{nftables.TableFamilyIPv4: errV4}, false},
		{"none", map[nftables.TableFamily]error{
			nftables.TableFamilyIPv4: errV4,
			nftables.TableFamilyIPv6: errV6,
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fail := tc.fail
			checksumFillFamily = func(
				_ *nftables.Conn,
				family nftables.TableFamily,
				_ string,
				_ bool,
			) error {
				return fail[family]
			}
			err := SetTCPChecksumFill("cm-test", true)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}

	if err := SetTCPChecksumFill("bad name", true); err == nil {
		t.Fatal("accepted invalid interface name")
	}
}

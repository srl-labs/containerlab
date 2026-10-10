package nftables

import (
	"errors"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
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

func TestSetTCPChecksumFillFamily(t *testing.T) {
	orig := checksumFillFamily
	t.Cleanup(func() { checksumFillFamily = orig })

	var got nftables.TableFamily
	wantErr := errors.New("checksum unavailable")
	checksumFillFamily = func(
		_ *nftables.Conn,
		family nftables.TableFamily,
		_ string,
		_ bool,
	) error {
		got = family
		return wantErr
	}
	for _, tc := range []struct {
		name string
		af   int
		want nftables.TableFamily
	}{
		{"IPv4", unix.AF_INET, nftables.TableFamilyIPv4},
		{"IPv6", unix.AF_INET6, nftables.TableFamilyIPv6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := SetTCPChecksumFill("cm-test", tc.af, true); !errors.Is(err, wantErr) {
				t.Fatalf("error = %v; want %v", err, wantErr)
			}
			if got != tc.want {
				t.Fatalf("family = %v; want %v", got, tc.want)
			}
		})
	}

	if err := SetTCPChecksumFill("bad name", unix.AF_INET, true); err == nil {
		t.Fatal("accepted invalid interface name")
	}
	if err := SetTCPChecksumFill("cm-test", unix.AF_UNSPEC, true); err == nil {
		t.Fatal("accepted unsupported address family")
	}
}

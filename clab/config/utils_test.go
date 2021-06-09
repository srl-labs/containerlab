package config

import (
	"testing"

	"inet.af/netaddr"
)

func TestFarEndIP(t *testing.T) {

	lst := map[string]string{
		"10.0.0.1/32": "",

		"10.0.0.0/31": "10.0.0.1/31",
		"10.0.0.1/31": "10.0.0.0/31",
		"10.0.0.2/31": "10.0.0.3/31",
		"10.0.0.3/31": "10.0.0.2/31",

		"10.0.0.1/30": "10.0.0.2/30",
		"10.0.0.2/30": "10.0.0.1/30",
		"10.0.0.0/30": "",
		"10.0.0.3/30": "",
		"10.0.0.4/30": "",
		"10.0.0.5/30": "10.0.0.6/30",
		"10.0.0.6/30": "10.0.0.5/30",
	}

	for k, v := range lst {
		n := ipFarEnd(netaddr.MustParseIPPrefix(k))
		if n.IsZero() && v == "" {
			continue
		}
		if v != n.String() {
			t.Errorf("far end of %s, got %s, expected %s", k, n, v)
		}
	}

}

func TestIPLastOctect(t *testing.T) {

	lst := map[string]int{
		"10.0.0.1/32": 1,
		"::1/32":      1,
	}
	for k, v := range lst {
		n := netaddr.MustParseIPPrefix(k)
		lo := ipLastOctet(n.IP())
		if v != lo {
			t.Errorf("far end of %s, got %d, expected %d", k, lo, v)
		}
	}

}

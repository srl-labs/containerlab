package config

import (
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/srl-labs/containerlab/types"
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
		n2, _ := ipFarEndS(k)

		if n.IsZero() && v == "" && n2 == "" {
			continue
		}

		if n2 != n.String() {
			t.Errorf("far end %s != %s, expected %s", n, n2, v)
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

func gettestLink() *types.Link {
	return &types.Link{
		A: &types.Endpoint{
			Node: &types.NodeConfig{
				ShortName: "a",
				Config: &types.ConfigDispatcher{
					Vars: map[string]interface{}{
						vkSystemIP: "10.0.0.1/32",
					},
				},
			},
		},
		B: &types.Endpoint{
			Node: &types.NodeConfig{
				ShortName: "b",
				Config: &types.ConfigDispatcher{
					Vars: map[string]interface{}{
						vkSystemIP: "10.0.0.2/32",
					},
				},
			},
		},
		Vars: map[string]interface{}{},
	}
}

func assert(t *testing.T, val, exp interface{}) {
	if !cmp.Equal(val, exp) {
		_, fn, line, _ := runtime.Caller(1)
		t.Errorf("assert failed on line %v in %s\n%s", line, fn, cmp.Diff(val, exp))
	}
}

func TestLinkName(t *testing.T) {
	l := gettestLink()
	n1, n2, _ := linkName(l)
	assert(t, n1, "to_b")
	assert(t, n2, "to_a")

	l.Vars[vkLinkNum] = 1
	n1, n2, _ = linkName(l)
	assert(t, n1, "to_b_1")
	assert(t, n2, "to_a_1")
}

func TestLinkIP(t *testing.T) {
	l := gettestLink()
	n1, n2, _ := linkIP(l)
	assert(t, n1, "1.1.2.0/31")
	assert(t, n2, "1.1.2.1/31")

	l.Vars[vkLinkNum] = 1
	n1, n2, _ = linkIP(l)
	assert(t, n1, "1.1.2.2/31")
	assert(t, n2, "1.1.2.3/31")
}

func TestPrepareLinkVars(t *testing.T) {
	a := make(Dict)
	b := make(Dict)
	l := gettestLink()
	_ = prepareLinkVars(l, a, b)
	assert(t, a, Dict{
		vkFarEnd:   Dict{vkLinkIP: "1.1.2.1/31", vkLinkName: "to_a", vkNodeName: "b"},
		vkLinkIP:   "1.1.2.0/31",
		vkLinkName: "to_b",
	})
	assert(t, b, Dict{
		vkFarEnd:   Dict{vkLinkIP: "1.1.2.0/31", vkLinkName: "to_b", vkNodeName: "a"},
		vkLinkIP:   "1.1.2.1/31",
		vkLinkName: "to_a",
	})

	l.Vars[vkLinkIP] = []string{"1.1.2.0/16", "1.1.2.1/16"}
	l.Vars[vkLinkName] = "the_same"

	_ = prepareLinkVars(l, a, b)
	assert(t, a, Dict{
		vkFarEnd:   Dict{vkLinkIP: "1.1.2.1/16", vkLinkName: "the_same", vkNodeName: "b"},
		vkLinkIP:   "1.1.2.0/16",
		vkLinkName: "the_same",
	})
	assert(t, b, Dict{
		vkFarEnd:   Dict{vkLinkIP: "1.1.2.0/16", vkLinkName: "the_same", vkNodeName: "a"},
		vkLinkIP:   "1.1.2.1/16",
		vkLinkName: "the_same",
	})
}

func TestIPfarEndS(t *testing.T) {
	ipA := "10.0.3.0/31"
	feA, err := ipFarEndS(ipA)
	assert(t, err, nil)
	assert(t, feA, "10.0.3.1/31")

	ipA = "10.0.3.0/30"
	feA, err = ipFarEndS(ipA)
	assert(t, err.Error(), "invalid ip 10.0.3.0/30 - zero IPPrefix")
	assert(t, feA, "")
}

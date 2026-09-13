package types

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestDADValidation(t *testing.T) {
	for _, dad := range []*bool{nil, new(true), new(false)} {
		m := &MgmtNet{IPAM: MgmtIPAM{DAD: dad}}
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
		if m.IPAM.DADEnabled() != (dad == nil || *dad) {
			t.Fatal("incorrect DAD default")
		}
	}
	if (&MgmtNet{IPAM: MgmtIPAM{Provider: "index"}}).Validate() == nil {
		t.Fatal("accepted invalid provider")
	}
}

func TestManagementIPAMYAML(t *testing.T) {
	var m MgmtNet
	if err := yaml.UnmarshalStrict(
		[]byte("ipam:\n  provider: runtime\n  dad: true\n"),
		&m,
	); err != nil {
		t.Fatal(err)
	}
	if m.IPAM.Provider != IPAMProviderRuntime || !m.IPAM.DADEnabled() {
		t.Fatalf("incorrect IPAM config: %+v", m.IPAM)
	}
}

package types

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestMgmtNetworksUnmarshalYAML(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  []MgmtNet
	}{
		{
			name:  "single",
			input: "network: clab\nipv4-subnet: 192.0.2.0/24\n",
			want:  []MgmtNet{{Network: "clab", IPv4Subnet: "192.0.2.0/24"}},
		},
		{
			name: "list",
			input: "- network: clab\n" +
				"- network: oob\n  ipv4_subnet: 198.51.100.0/24\n",
			want: []MgmtNet{
				{Network: "clab"},
				{Network: "oob", IPv4Subnet: "198.51.100.0/24"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got MgmtNetworks
			if err := yaml.UnmarshalStrict([]byte(tc.input), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d networks; want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i].Network != tc.want[i].Network ||
					got[i].IPv4Subnet != tc.want[i].IPv4Subnet {
					t.Fatalf("network %d = %+v; want %+v", i, *got[i], tc.want[i])
				}
			}
		})
	}
}

func TestMgmtNetworksUnmarshalYAMLReportsListFieldErrors(t *testing.T) {
	var got MgmtNetworks
	err := yaml.UnmarshalStrict([]byte("- network: oob\n  netwrk: x\n"), &got)
	if err == nil || !strings.Contains(err.Error(), "netwrk") {
		t.Fatalf("UnmarshalStrict() error = %v; want unknown field netwrk", err)
	}
}

func TestMgmtNetworksMarshalYAML(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   MgmtNetworks
		want string
	}{
		{name: "single", in: MgmtNetworks{{Network: "clab"}}, want: "network: clab\n"},
		{
			name: "list",
			in:   MgmtNetworks{{Network: "main"}, {Network: "oob"}},
			want: "- network: main\n- network: oob\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := yaml.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tc.want {
				t.Fatalf("Marshal() = %q; want %q", out, tc.want)
			}
		})
	}
}

func TestMgmtNetworksMarshalJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   MgmtNetworks
		want string
	}{
		{name: "single", in: MgmtNetworks{{Network: "clab"}}, want: `{"network":"clab","ipam":{}}`},
		{
			name: "list",
			in:   MgmtNetworks{{Network: "main"}, {Network: "oob"}},
			want: `[{"network":"main","ipam":{}},{"network":"oob","ipam":{}}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tc.want {
				t.Fatalf("Marshal() = %s; want %s", out, tc.want)
			}
		})
	}
}

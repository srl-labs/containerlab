package links

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

func TestNetemRawResolve(t *testing.T) {
	tests := map[string]struct {
		raw     *NetemRaw
		want    *Netem
		wantErr string
	}{
		"nothing set": {
			raw: &NetemRaw{},
		},
		"no impairment": {
			raw: nil,
		},
		"every impairment": {
			raw: &NetemRaw{Delay: "50ms", Jitter: "5ms", Loss: 0.5, Rate: 10000, Corruption: 0.1},
			want: &Netem{
				Delay:      50 * time.Millisecond,
				Jitter:     5 * time.Millisecond,
				Loss:       0.5,
				Rate:       10000,
				Corruption: 0.1,
			},
		},
		"delay that is not a duration": {
			raw:     &NetemRaw{Delay: "50"},
			wantErr: `netem delay "50" is not a duration`,
		},
		"negative jitter": {
			raw:     &NetemRaw{Delay: "50ms", Jitter: "-5ms"},
			wantErr: "must not be negative",
		},
		"jitter without delay": {
			raw:     &NetemRaw{Jitter: "5ms"},
			wantErr: "jitter cannot be set without setting delay",
		},
		"jitter the kernel would cap": {
			raw:     &NetemRaw{Delay: "5m", Jitter: "1m"},
			wantErr: "netem jitter must be at most 2.147483647s",
		},
		"loss above 100": {
			raw:     &NetemRaw{Loss: 101},
			wantErr: "loss must be in the range between 0 and 100",
		},
		"loss that is not a number": {
			raw:     &NetemRaw{Loss: math.NaN()},
			wantErr: "loss must be in the range between 0 and 100",
		},
		"rate whose bytes/s overflow": {
			raw:     &NetemRaw{Rate: math.MaxUint64},
			wantErr: "netem rate must be at most",
		},
		"negative corruption": {
			raw:     &NetemRaw{Corruption: -1},
			wantErr: "corruption must be in the range between 0 and 100",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tc.raw.Resolve()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Fatalf("netem mismatch (-want +got):\n%s", d)
			}
		})
	}
}

// resolveNetemLink reads a link definition and resolves it between two fake nodes.
func resolveNetemLink(t *testing.T, definition string) (Link, error) {
	t.Helper()

	var ld LinkDefinition
	if err := yaml.Unmarshal([]byte(definition), &ld); err != nil {
		return nil, err
	}

	return ld.Link.Resolve(&ResolveParams{Nodes: map[string]Node{
		"r1": newFakeNode("r1"),
		"r2": newFakeNode("r2"),
	}})
}

func TestNetemOfLinkEndpoints(t *testing.T) {
	tests := map[string]string{
		"brief link": `
endpoints: ["r1:eth1", "r2:eth1"]
netem:
  - delay: 50ms
    loss: 1
  - {}
`,
		"extended link": `
type: veth
endpoints:
  - node: r1
    interface: eth1
    netem:
      delay: 50ms
      loss: 1
  - node: r2
    interface: eth1
`,
		"veth-stitch link": `
type: veth-stitch
endpoints:
  - node: r1
    interface: eth1
    netem:
      delay: 50ms
      loss: 1
  - node: r2
    interface: eth1
`,
	}

	for name, definition := range tests {
		t.Run(name, func(t *testing.T) {
			link, err := resolveNetemLink(t, definition)
			if err != nil {
				t.Fatal(err)
			}

			endpoints := link.GetEndpoints()
			want := &Netem{Delay: 50 * time.Millisecond, Loss: 1}
			if d := cmp.Diff(want, endpoints[0].GetNetem()); d != "" {
				t.Fatalf("netem of r1:eth1 mismatch (-want +got):\n%s", d)
			}
			if endpoints[1].GetNetem() != nil {
				t.Fatalf("r2:eth1 must not be impaired, got %+v", endpoints[1].GetNetem())
			}
		})
	}
}

func TestNetemOfLinkEndpointsIsValidated(t *testing.T) {
	tests := map[string]struct {
		definition string
		wantErr    string
	}{
		"invalid impairment": {
			definition: `
endpoints: ["r1:eth1", "r2:eth1"]
netem:
  - jitter: 5ms
`,
			wantErr: "invalid netem for r1:eth1",
		},
		"more impairments than endpoints": {
			definition: `
endpoints: ["r1:eth1", "r2:eth1"]
netem: [{delay: 1ms}, {delay: 1ms}, {delay: 1ms}]
`,
			wantErr: "number of netem entries exceeds the number of endpoints",
		},
		"a link that isn't a veth link": {
			definition: `
endpoints: ["r1:eth1", "host:r1-eth1"]
netem: [{delay: 1ms}]
`,
			wantErr: "netem is only supported on veth links",
		},
		"an extended link that isn't a veth link": {
			definition: `
type: macvlan
host-interface: eth0
endpoint:
  node: r1
  interface: eth1
  netem:
    delay: 1ms
`,
			wantErr: "netem is only supported on veth links, not on macvlan links",
		},
		"a host endpoint of an extended veth link": {
			definition: `
type: veth
endpoints:
  - node: r1
    interface: eth1
  - node: host
    interface: r1-eth1
    netem:
      delay: 1ms
`,
			wantErr: "netem is not supported on host endpoints",
		},
		"a mgmt-net endpoint of a veth-stitch link": {
			definition: `
type: veth-stitch
endpoints:
  - node: r1
    interface: eth1
  - node: mgmt-net
    interface: r1-eth1
    netem:
      delay: 1ms
`,
			wantErr: "netem is not supported on mgmt-net endpoints",
		},
		"an invalid impairment of a veth-stitch link": {
			definition: `
type: veth-stitch
endpoints:
  - node: r1
    interface: eth1
    netem:
      loss: 101
  - node: r2
    interface: eth1
`,
			wantErr: "invalid netem for r1:eth1",
		},
		"a vxlan-stitch link": {
			definition: `
type: vxlan-stitch
remote: 192.0.2.1
vni: 100
endpoint:
  node: r1
  interface: eth1
  netem:
    delay: 1ms
`,
			wantErr: "netem is only supported on veth links, not on vxlan-stitch links",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := resolveNetemLink(t, tc.definition)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNetemEndpointOfVEthStitchLink(t *testing.T) {
	var ld LinkDefinition
	if err := yaml.Unmarshal([]byte(`
type: veth-stitch
endpoints:
  - node: r1
    interface: eth1
  - node: r2
    interface: eth2
`), &ld); err != nil {
		t.Fatal(err)
	}

	link, err := ld.Link.Resolve(&ResolveParams{LabName: "lab", Nodes: map[string]Node{
		"r1": newFakeNode("r1"),
		"r2": newFakeNode("r2"),
	}})
	if err != nil {
		t.Fatal(err)
	}

	endpoints := link.GetEndpoints()
	wantIfaces := []string{
		stitchFarEndName("lab", "r2", "eth2"),
		stitchFarEndName("lab", "r1", "eth1"),
	}
	for i, endpoint := range endpoints {
		target := NetemEndpoint(endpoint)
		if target.GetNode() != GetHostLinkNode() || target.GetIfaceName() != wantIfaces[i] {
			t.Fatalf("netem endpoint of %s = %s, want host:%s", endpoint, target, wantIfaces[i])
		}
	}
}

func TestNetemNeedsVEthStitch(t *testing.T) {
	nodes := map[string]Node{
		"r1": newFakeNode("r1"),
		"srsim": &fakeDefaultLinkTypeNode{
			fakeNode: newFakeNode("srsim"),
			linkType: LinkTypeVethStitch,
		},
	}

	for linkType, want := range map[string][]bool{
		"veth":        {false, true},
		"veth-stitch": {false, false},
	} {
		t.Run(linkType, func(t *testing.T) {
			var ld LinkDefinition
			if err := yaml.Unmarshal([]byte(`
type: `+linkType+`
endpoints:
  - node: r1
    interface: eth1
  - node: srsim
    interface: eth1
`), &ld); err != nil {
				t.Fatal(err)
			}

			link, err := ld.Link.Resolve(&ResolveParams{LabName: "lab", Nodes: nodes})
			if err != nil {
				t.Fatal(err)
			}

			for i, endpoint := range link.GetEndpoints() {
				if got := NetemNeedsVEthStitch(endpoint); got != want[i] {
					t.Fatalf("NetemNeedsVEthStitch(%s) = %t, want %t", endpoint, got, want[i])
				}
			}
		})
	}
}

func TestNetemEndpointOfVEthLink(t *testing.T) {
	link, err := resolveNetemLink(t, `endpoints: ["r1:eth1", "r2:eth1"]`)
	if err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range link.GetEndpoints() {
		if target := NetemEndpoint(endpoint); target != endpoint {
			t.Fatalf("netem endpoint of %s = %s, want the endpoint itself", endpoint, target)
		}
	}
}

func TestLinkVEthRawToLinkBriefRawKeepsTheNetem(t *testing.T) {
	netem := &NetemRaw{Delay: "50ms"}
	raw := &LinkVEthRaw{Endpoints: []*EndpointRaw{
		{Node: "r1", Iface: "eth1"},
		{Node: "r2", Iface: "eth1", Netem: netem},
	}}

	brief := raw.ToLinkBriefRaw()

	if d := cmp.Diff([]*NetemRaw{nil, netem}, brief.Netem); d != "" {
		t.Fatalf("brief netem mismatch (-want +got):\n%s", d)
	}
	if unimpaired := (&LinkVEthRaw{Endpoints: raw.Endpoints[:1]}).ToLinkBriefRaw(); unimpaired.Netem != nil {
		t.Fatalf("a link without impairments must not get a netem list, got %+v", unimpaired.Netem)
	}
}

// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

//nolint:mnd // the demo labs are made-up figures from the first line to the last
package fake

import (
	"fmt"
	"strings"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	kindSRL   = "nokia_srlinux"
	kindSRSim = "nokia_srsim"
	kindCEOS  = "arista_ceos"
	kindLinux = "linux"
	kindCRPD  = "juniper_crpd"
	kindIOL   = "cisco_iol"

	kindFortigate = "fortinet_fortigate"

	imageSRL   = "ghcr.io/nokia/srlinux:25.10.1"
	imageSRSim = "registry.srlinux.dev/pub/nokia_srsim:25.10.R1"
	imageCEOS  = "ceos:4.34.2F"
	imageCRPD  = "crpd:24.4R1.9"
	imageIOL   = "vrnetlab/cisco_iol:L2-17.12.01"
	imageForti = "vrnetlab/fortinet_fortigate:7.4.4"
	imageLinux = "ghcr.io/srl-labs/network-multitool:latest"

	stateRunning = "running"
	stateExited  = "exited"

	demoOwner = "demo"
	demoHome  = "/home/demo"

	mtuFabric = 9232
	mtuMgmt   = 1500
)

// nodeSpec describes a node of a demo lab.
type nodeSpec struct {
	name   string
	kind   string
	typ    string
	group  string
	role   string
	level  int
	state  string
	status string
}

// linkSpec describes a link of a demo lab and the traffic that flows over it.
type linkSpec struct {
	a, aif string
	z, zif string
	// rate is the traffic the simulation starts with, in bits per second.
	rate float64
	down bool
	// netem is set on the A side of the link.
	netem *clabinternaltuibackend.Impairment
}

type labSpec struct {
	name     string
	topoFile string
	deployed bool
	nodes    []nodeSpec
	links    []linkSpec
	shares   []clabinternaltuibackend.Share
}

func up(since string) (state, status string) {
	return stateRunning, "Up " + since
}

// fabricSpec generates a leaf-spine fabric with hosts behind every leaf. It is large on
// purpose, to show how the UI copes with labs that do not fit a screen.
func fabricSpec(leaves int, deployed bool) labSpec {
	const (
		spines       = 4
		hostsPerLeaf = 4
		baseRate     = 4e5
	)

	state, status := "", ""
	if deployed {
		state, status = up("2 hours")
	}

	spec := labSpec{
		name:     "fabric",
		topoFile: demoHome + "/labs/fabric/fabric.clab.yml",
		deployed: deployed,
	}

	for s := range spines {
		spec.nodes = append(spec.nodes, nodeSpec{
			fmt.Sprintf("spine%d", s+1), kindSRL, "ixrd3l", "spine", "spine", 1, state, status,
		})
	}

	for l := range leaves {
		leaf := fmt.Sprintf("leaf%d", l+1)
		spec.nodes = append(spec.nodes, nodeSpec{
			leaf, kindSRL, "ixrd2l", "leaf", "leaf", 2, state, status,
		})

		for s := range spines {
			spec.links = append(spec.links, linkSpec{
				a: leaf, aif: fmt.Sprintf("e1-%d", 49+s),
				z: fmt.Sprintf("spine%d", s+1), zif: fmt.Sprintf("e1-%d", l+1),
				rate: baseRate * float64(1+(l+s)%5),
			})
		}

		for h := range hostsPerLeaf {
			host := fmt.Sprintf("h%d-%d", l+1, h+1)
			spec.nodes = append(spec.nodes, nodeSpec{
				host, kindLinux, "", "server", "server", 3, state, status,
			})
			spec.links = append(spec.links, linkSpec{
				a: host, aif: "eth1", z: leaf, zif: fmt.Sprintf("e1-%d", h+1),
				rate: baseRate / 4 * float64(1+h),
			})
		}
	}

	return spec
}

// demoFabricLeaves is the size of the fabric the demo offers for deployment.
const demoFabricLeaves = 32

// demoSpecs returns the labs of the demo. They include a large fabric, which is not
// deployed unless a size is asked for.
func demoSpecs(fabricLeaves int) []labSpec {
	if fabricLeaves > 0 {
		return append(baseSpecs(), fabricSpec(fabricLeaves, true))
	}

	return append(baseSpecs(), fabricSpec(demoFabricLeaves, false))
}

func baseSpecs() []labSpec {
	return []labSpec{
		datacenterSpec(),
		providerSpec(),
		metroSpec(),
		campusSpec(),
		stoppedSpec(),
		{
			name:     "clos",
			topoFile: demoHome + "/labs/clos.clab.yml",
			nodes: []nodeSpec{
				{name: "spine1", kind: kindSRL, role: "spine", level: 1},
				{name: "spine2", kind: kindSRL, role: "spine", level: 1},
				{name: "leaf1", kind: kindSRL, role: "leaf", level: 2},
				{name: "leaf2", kind: kindSRL, role: "leaf", level: 2},
				{name: "leaf3", kind: kindSRL, role: "leaf", level: 2},
				{name: "leaf4", kind: kindSRL, role: "leaf", level: 2},
			},
			links: []linkSpec{
				{a: "leaf1", aif: "e1-49", z: "spine1", zif: "e1-1"},
				{a: "leaf1", aif: "e1-50", z: "spine2", zif: "e1-1"},
				{a: "leaf2", aif: "e1-49", z: "spine1", zif: "e1-2"},
				{a: "leaf2", aif: "e1-50", z: "spine2", zif: "e1-2"},
				{a: "leaf3", aif: "e1-49", z: "spine1", zif: "e1-3"},
				{a: "leaf3", aif: "e1-50", z: "spine2", zif: "e1-3"},
				{a: "leaf4", aif: "e1-49", z: "spine1", zif: "e1-4"},
				{a: "leaf4", aif: "e1-50", z: "spine2", zif: "e1-4"},
			},
		},
		{
			name:     "linux",
			topoFile: demoHome + "/labs/linux.clab.yml",
			nodes: []nodeSpec{
				{name: "l1", kind: kindLinux},
				{name: "l2", kind: kindLinux},
			},
			links: []linkSpec{{a: "l1", aif: "eth1", z: "l2", zif: "eth1"}},
		},
	}
}

// datacenterSpec is a leaf-spine fabric with one leaf still booting, one link down and
// an impaired link.
func datacenterSpec() labSpec {
	run3h, st3h := up("3 hours")

	return labSpec{
		name:     "dc1",
		topoFile: demoHome + "/labs/dc1/dc1.clab.yml",
		deployed: true,
		nodes: []nodeSpec{
			{"spine1", kindSRL, "ixrd3l", "spine", "spine", 1, run3h, st3h},
			{"spine2", kindSRL, "ixrd3l", "spine", "spine", 1, run3h, st3h},
			{"leaf1", kindSRL, "ixrd2l", "leaf", "leaf", 2, run3h, st3h},
			{"leaf2", kindSRL, "ixrd2l", "leaf", "leaf", 2, run3h, st3h},
			{"leaf3", kindSRL, "ixrd2l", "leaf", "leaf", 2, run3h, st3h},
			{
				"leaf4", kindSRL, "ixrd2l", "leaf", "leaf", 2,
				stateRunning, "Up 12 seconds (health: starting)",
			},
			{"client1", kindLinux, "", "server", "client", 3, run3h, st3h},
			{"client2", kindLinux, "", "server", "client", 3, run3h, st3h},
			{"client3", kindLinux, "", "server", "client", 3, run3h, st3h},
			{"client4", kindLinux, "", "server", "client", 3, run3h, st3h},
		},
		links: []linkSpec{
			{
				a: "leaf1", aif: "e1-49", z: "spine1", zif: "e1-1", rate: 2.4e6,
				netem: &clabinternaltuibackend.Impairment{
					Delay: 50 * time.Millisecond, Jitter: 5 * time.Millisecond, LossPct: 1,
				},
			},
			{a: "leaf1", aif: "e1-50", z: "spine2", zif: "e1-1", down: true},
			{a: "leaf2", aif: "e1-49", z: "spine1", zif: "e1-2", rate: 5.8e6},
			{a: "leaf2", aif: "e1-50", z: "spine2", zif: "e1-2", rate: 4.1e6},
			{a: "leaf3", aif: "e1-49", z: "spine1", zif: "e1-3", rate: 18.4e6},
			{a: "leaf3", aif: "e1-50", z: "spine2", zif: "e1-3", rate: 16.2e6},
			{a: "leaf4", aif: "e1-49", z: "spine1", zif: "e1-4"},
			{a: "leaf4", aif: "e1-50", z: "spine2", zif: "e1-4"},
			{a: "client1", aif: "eth1", z: "leaf1", zif: "e1-1", rate: 1.2e6},
			{a: "client2", aif: "eth1", z: "leaf2", zif: "e1-1", rate: 9.6e4},
			{a: "client3", aif: "eth1", z: "leaf3", zif: "e1-1", rate: 12.3e6},
			{a: "client4", aif: "eth1", z: "leaf4", zif: "e1-1"},
		},
		shares: []clabinternaltuibackend.Share{{
			Tool: clabinternaltuibackend.ShareSSHX,
			Lab:  "dc1",
			Link: "https://sshx.io/s/Ab3xQk91#k9wPz2Lm",
		}},
	}
}

// providerSpec is a service provider network: a meshed core of P routers, PE routers
// homed to two of them, route reflectors and customer edge routers. One PE has crashed.
// The topology gives no hints about tiers, as is common: only the names do.
func providerSpec() labSpec {
	run, status := up("41 minutes (healthy)")
	plain, plainStatus := up("41 minutes")

	return labSpec{
		name:     "sp-core",
		topoFile: demoHome + "/labs/sp-core/sp-core.clab.yml",
		deployed: true,
		nodes: []nodeSpec{
			{name: "p1", kind: kindSRSim, typ: "sr-7", state: run, status: status},
			{name: "p2", kind: kindSRSim, typ: "sr-7", state: run, status: status},
			{name: "p3", kind: kindSRSim, typ: "sr-7", state: run, status: status},
			{name: "p4", kind: kindSRSim, typ: "sr-7", state: run, status: status},
			{name: "rr1", kind: kindCRPD, state: plain, status: plainStatus},
			{name: "rr2", kind: kindCRPD, state: plain, status: plainStatus},
			{name: "pe1", kind: kindSRSim, typ: "sr-1", state: run, status: status},
			{name: "pe2", kind: kindSRSim, typ: "sr-1", state: run, status: status},
			{name: "pe3", kind: kindSRSim, typ: "sr-1", state: run, status: status},
			{
				name: "pe4", kind: kindSRSim, typ: "sr-1",
				state: stateExited, status: "Exited (137) 4 minutes ago",
			},
			{name: "ce1", kind: kindLinux, state: plain, status: plainStatus},
			{name: "ce2", kind: kindLinux, state: plain, status: plainStatus},
			{name: "ce3", kind: kindLinux, state: plain, status: plainStatus},
			{name: "ce4", kind: kindLinux, state: plain, status: plainStatus},
		},
		links: []linkSpec{
			{a: "p1", aif: "1/1/c1/1", z: "p2", zif: "1/1/c1/1", rate: 9.4e6},
			{a: "p1", aif: "1/1/c2/1", z: "p3", zif: "1/1/c1/1", rate: 7.1e6},
			{a: "p2", aif: "1/1/c2/1", z: "p4", zif: "1/1/c1/1", rate: 6.3e6},
			{a: "p3", aif: "1/1/c2/1", z: "p4", zif: "1/1/c2/1", rate: 8.8e6},
			{a: "p1", aif: "1/1/c3/1", z: "p4", zif: "1/1/c3/1", rate: 2.2e6},
			{a: "p2", aif: "1/1/c3/1", z: "p3", zif: "1/1/c3/1", rate: 1.9e6},
			{a: "rr1", aif: "eth1", z: "p1", zif: "1/1/c4/1", rate: 4.0e4},
			{a: "rr2", aif: "eth1", z: "p4", zif: "1/1/c4/1", rate: 3.6e4},
			{a: "pe1", aif: "1/1/c1/1", z: "p1", zif: "1/1/c5/1", rate: 3.2e6},
			{a: "pe1", aif: "1/1/c2/1", z: "p2", zif: "1/1/c5/1", rate: 2.9e6},
			{a: "pe2", aif: "1/1/c1/1", z: "p1", zif: "1/1/c6/1", rate: 1.4e6},
			{a: "pe2", aif: "1/1/c2/1", z: "p2", zif: "1/1/c6/1", rate: 1.6e6},
			{a: "pe3", aif: "1/1/c1/1", z: "p3", zif: "1/1/c5/1", rate: 4.4e6},
			{a: "pe3", aif: "1/1/c2/1", z: "p4", zif: "1/1/c5/1", rate: 4.0e6},
			{a: "pe4", aif: "1/1/c1/1", z: "p3", zif: "1/1/c6/1", down: true},
			{a: "pe4", aif: "1/1/c2/1", z: "p4", zif: "1/1/c6/1", down: true},
			{a: "ce1", aif: "eth1", z: "pe1", zif: "1/1/c3/1", rate: 2.1e6},
			{a: "ce2", aif: "eth1", z: "pe1", zif: "1/1/c4/1", rate: 6.0e5},
			{a: "ce2", aif: "eth2", z: "pe2", zif: "1/1/c3/1", rate: 5.5e5},
			{a: "ce3", aif: "eth1", z: "pe3", zif: "1/1/c3/1", rate: 3.9e6},
			{a: "ce4", aif: "eth1", z: "pe4", zif: "1/1/c3/1", down: true},
		},
	}
}

// metroSpec is a metro ring of eight routers with customer premises equipment hanging
// off every other one.
func metroSpec() labSpec {
	const ringSize = 8

	run, status := up("6 hours")
	spec := labSpec{
		name:     "metro",
		topoFile: demoHome + "/labs/metro/metro-ring.clab.yml",
		deployed: true,
	}

	for i := range ringSize {
		spec.nodes = append(spec.nodes, nodeSpec{
			name: fmt.Sprintf("r%d", i+1), kind: kindSRL, typ: "ixr6e", state: run, status: status,
		})
	}

	for i := range ringSize {
		next := (i + 1) % ringSize
		spec.links = append(spec.links, linkSpec{
			a: fmt.Sprintf("r%d", i+1), aif: "e1-1",
			z: fmt.Sprintf("r%d", next+1), zif: "e1-2",
			rate: 1.5e6 * float64(1+i%3),
		})
	}

	for i := 1; i < ringSize; i += 2 {
		cpe := fmt.Sprintf("cpe%d", (i+1)/2) //nolint:mnd // one for every other router
		spec.nodes = append(spec.nodes, nodeSpec{
			name: cpe, kind: kindLinux, state: run, status: status,
		})
		spec.links = append(spec.links, linkSpec{
			a: cpe, aif: "eth1", z: fmt.Sprintf("r%d", i+1), zif: "e1-3", rate: 4.2e5,
		})
	}

	return spec
}

// campusSpec is a campus network: a core pair, two distribution pairs with access
// switches homed to both of their pair, and a firewall towards the internet.
func campusSpec() labSpec {
	run, status := up("2 days")

	spec := labSpec{
		name:     "campus",
		topoFile: demoHome + "/labs/campus/campus.clab.yml",
		deployed: true,
		nodes: []nodeSpec{
			{name: "internet", kind: kindLinux, state: run, status: status},
			{name: "fw1", kind: kindFortigate, state: run, status: status},
			{name: "core1", kind: kindCEOS, state: run, status: status},
			{name: "core2", kind: kindCEOS, state: run, status: status},
		},
		links: []linkSpec{
			{a: "fw1", aif: "port1", z: "internet", zif: "eth1", rate: 7.5e6},
			{a: "core1", aif: "eth1", z: "fw1", zif: "port2", rate: 4.1e6},
			{a: "core2", aif: "eth1", z: "fw1", zif: "port3", rate: 3.4e6},
			{a: "core1", aif: "eth2", z: "core2", zif: "eth2", rate: 9.0e5},
		},
	}

	const (
		distSwitches  = 4
		accessPerPair = 3
	)

	for d := range distSwitches {
		dist := fmt.Sprintf("dist%d", d+1)
		spec.nodes = append(
			spec.nodes,
			nodeSpec{name: dist, kind: kindCEOS, state: run, status: status},
		)

		for c := range 2 {
			spec.links = append(spec.links, linkSpec{
				a: dist, aif: fmt.Sprintf("eth%d", c+1),
				z: fmt.Sprintf("core%d", c+1), zif: fmt.Sprintf("eth%d", d+3),
				rate: 6.0e5 * float64(1+(d+c)%3),
			})
		}
	}

	for pair := range distSwitches / 2 {
		for a := range accessPerPair {
			access := fmt.Sprintf("acc%d", pair*accessPerPair+a+1)
			spec.nodes = append(spec.nodes, nodeSpec{
				name: access, kind: kindIOL, typ: "l2", state: run, status: status,
			})

			for side := range 2 {
				spec.links = append(spec.links, linkSpec{
					a: access, aif: fmt.Sprintf("e0/%d", side),
					z: fmt.Sprintf("dist%d", pair*2+side+1), zif: fmt.Sprintf("eth%d", a+3),
					rate: 1.8e5 * float64(1+a),
				})
			}
		}
	}

	return spec
}

// stoppedSpec is a lab whose nodes were stopped.
func stoppedSpec() labSpec {
	const exited = "Exited (0) 2 days ago"

	return labSpec{
		name:     "evpn-poc",
		topoFile: demoHome + "/labs/evpn-poc/evpn.clab.yml",
		deployed: true,
		nodes: []nodeSpec{
			{"core1", kindCEOS, "", "core", "spine", 1, stateExited, exited},
			{"acc1", kindCEOS, "", "access", "leaf", 2, stateExited, exited},
			{"acc2", kindCEOS, "", "access", "leaf", 2, stateExited, exited},
		},
		links: []linkSpec{
			{a: "acc1", aif: "eth1", z: "core1", zif: "eth1", down: true},
			{a: "acc2", aif: "eth1", z: "core1", zif: "eth2", down: true},
		},
	}
}

func imageFor(kind string) string {
	switch kind {
	case kindSRL:
		return imageSRL
	case kindSRSim:
		return imageSRSim
	case kindCEOS:
		return imageCEOS
	case kindCRPD:
		return imageCRPD
	case kindIOL:
		return imageIOL
	case kindFortigate:
		return imageForti
	default:
		return imageLinux
	}
}

func containerName(lab, node string) string {
	return "clab-" + lab + "-" + node
}

// ifaceAlias returns the name the node's operating system uses for an interface.
func ifaceAlias(kind, name string) string {
	if kind != kindSRL {
		return ""
	}

	if rest, ok := strings.CutPrefix(name, "e"); ok {
		return "ethernet-" + strings.ReplaceAll(rest, "-", "/")
	}

	return ""
}

func mgmtInterface(kind string) string {
	if kind == kindSRL {
		return "mgmt0"
	}

	return "eth0"
}

func demoPorts(kind string) []clabinternaltuibackend.Port {
	const tcp = "tcp"

	switch kind {
	case kindSRL, kindSRSim:
		return []clabinternaltuibackend.Port{
			{ContainerPort: 22, Protocol: tcp},
			{ContainerPort: 830, Protocol: tcp},
			{ContainerPort: 57400, Protocol: tcp},
		}
	case kindCEOS:
		return []clabinternaltuibackend.Port{
			{ContainerPort: 22, Protocol: tcp},
			{ContainerPort: 6030, Protocol: tcp},
		}
	default:
		return []clabinternaltuibackend.Port{
			{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: tcp},
		}
	}
}

func buildNode(lab *labSpec, labIndex, i int, spec *nodeSpec) clabinternaltuibackend.Node {
	node := clabinternaltuibackend.Node{
		NodeRef: clabinternaltuibackend.NodeRef{
			Lab:       lab.name,
			Name:      spec.name,
			Container: containerName(lab.name, spec.name),
		},
		ID:     fmt.Sprintf("%012x", 0x3f9c2a71b0e4+labIndex*4096+i*257),
		Kind:   spec.kind,
		Type:   spec.typ,
		Group:  spec.group,
		Image:  imageFor(spec.kind),
		State:  spec.state,
		Status: spec.status,
		Owner:  demoOwner,
		Ports:  demoPorts(spec.kind),
		Labels: map[string]string{
			"containerlab":   lab.name,
			"clab-node-name": spec.name,
			"clab-node-kind": spec.kind,
			"clab-topo-file": lab.topoFile,
			"clab-owner":     demoOwner,
		},
	}

	if spec.state == stateRunning {
		const firstHost = 11

		node.Pid = 4100 + labIndex*100 + i
		node.IPv4 = fmt.Sprintf("172.20.%d.%d/24", 20+labIndex, firstHost+i)
		node.IPv6 = fmt.Sprintf("3fff:172:20:%d::%x/64", 20+labIndex, firstHost+i)
	}

	return node
}

func buildTopology(lab *labSpec) clabinternaltuibackend.Topology {
	topo := clabinternaltuibackend.Topology{Lab: lab.name}

	for i := range lab.nodes {
		n := &lab.nodes[i]
		topo.Nodes = append(topo.Nodes, clabinternaltuibackend.TopoNode{
			Name:  n.name,
			Kind:  n.kind,
			Group: n.group,
			Role:  n.role,
			Level: n.level,
		})
	}

	for i := range lab.links {
		l := &lab.links[i]
		topo.Links = append(topo.Links, clabinternaltuibackend.Link{
			A: clabinternaltuibackend.Endpoint{Node: l.a, Interface: l.aif},
			Z: &clabinternaltuibackend.Endpoint{Node: l.z, Interface: l.zif},
		})
	}

	return topo
}

func kindOf(lab *labSpec, node string) string {
	for i := range lab.nodes {
		if lab.nodes[i].name == node {
			return lab.nodes[i].kind
		}
	}

	return kindLinux
}

func demoMAC(labIndex, link int, zSide bool) string {
	side := 1
	if zSide {
		side = 2
	}

	return fmt.Sprintf("1a:b0:%02x:ff:%02x:%02x", labIndex, link, side)
}

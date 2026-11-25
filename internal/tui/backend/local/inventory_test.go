// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

// newRuntimeBackend returns a backend on top of a mocked container runtime.
func newRuntimeBackend(
	t *testing.T,
	adjust ...func(o *Options, d *deps),
) (*testBackend, *clabmocksmockruntime.MockContainerRuntime) {
	t.Helper()

	rt := clabmocksmockruntime.NewMockContainerRuntime(gomock.NewController(t))

	adjust = append([]func(o *Options, d *deps){func(_ *Options, d *deps) {
		d.rt = rt
		d.clab = &clabcore.CLab{
			Runtimes: map[string]clabruntime.ContainerRuntime{testRuntime: rt},
		}
	}}, adjust...)

	return newTestBackend(t, adjust...), rt
}

// labContainers returns what a runtime lists for two labs: a running one with a node that
// uses a network namespace holder and terminal sharing attached, and a stopped one.
func labContainers(topo string) []clabruntime.GenericContainer {
	labDir := filepath.Join(filepath.Dir(topo), "clab-lab1")

	leaf10 := nodeContainer("lab1", "leaf10", topo)
	leaf10.Labels[clabconstants.NodeLabDir] = filepath.Join(labDir, "leaf10")
	leaf10.Labels[clabconstants.Owner] = "alice"
	leaf10.Labels[clabconstants.NodeType] = "ixr-d2l"
	leaf10.Labels[clabconstants.NodeGroup] = "leaves"
	leaf10.Status = "Up 3 hours (healthy)"
	leaf10.NetworkName = "clab"
	leaf10.NetworkSettings = clabruntime.GenericMgmtIPs{
		IPv4addr: "172.20.20.10", IPv4pLen: 24, IPv4Gw: "172.20.20.1",
		IPv6addr: "3fff:172:20:20::a", IPv6pLen: 64,
	}
	leaf10.Ports = []*clabtypes.GenericPortBinding{
		{HostIP: "::", HostPort: 50057, ContainerPort: 57400, Protocol: "tcp"},
		{ContainerPort: 22, Protocol: "tcp"},
		{HostIP: "0.0.0.0", HostPort: 50057, ContainerPort: 57400, Protocol: "tcp"},
		nil,
		{ContainerPort: 22, Protocol: "tcp"},
	}

	// leaf2 has no addressing of its own: it lives in the network namespace of a holder.
	leaf2 := nodeContainer("lab1", "leaf2", topo)
	leaf2.Status = "Up 3 hours (unhealthy)"
	leaf2.Pid = 4300

	holder := nodeContainer("lab1", "leaf2-netns", topo)
	holder.Labels[clabconstants.InternalNode] = "true"
	holder.Labels[clabconstants.RootNodeLongName] = "clab-lab1-leaf2"
	holder.NetworkName = "clab"
	holder.NetworkSettings = clabruntime.GenericMgmtIPs{IPv4addr: "172.20.20.2", IPv4pLen: 24}
	holder.Ports = []*clabtypes.GenericPortBinding{{ContainerPort: 830, Protocol: "tcp"}}

	sshx := nodeContainer("lab1", "sshx", topo)
	sshx.ID = "sshx-container-id"
	sshx.Labels[clabconstants.ToolType] = "sshx"

	gotty := nodeContainer("lab1", "gotty", topo)
	gotty.Labels[clabconstants.ToolType] = "gotty"
	gotty.Ports = []*clabtypes.GenericPortBinding{
		{ContainerPort: 9000, Protocol: "tcp"},
		{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"},
	}

	// A tool the backend does not know is neither a node nor a sharing session.
	other := nodeContainer("lab1", "other", topo)
	other.Labels[clabconstants.ToolType] = "api-server"

	stopped := nodeContainer("lab0", "r1", "/gone/lab0.clab.yml")
	stopped.Names = []string{"/clab-lab0-r1"}
	stopped.State = "exited"
	stopped.Status = "Exited (137) 2 days ago"
	stopped.Pid = 0
	delete(stopped.Labels, clabconstants.NodeName)

	// A tool left behind by a lab that is gone does not make a lab.
	orphan := nodeContainer("ghost", "sshx", topo)
	orphan.Labels[clabconstants.ToolType] = "sshx"
	orphan.State = "exited"

	return []clabruntime.GenericContainer{
		leaf10, sshx, holder, stopped, leaf2, gotty, other, orphan,
	}
}

func TestInventoryFromRuntime(t *testing.T) {
	topo := filepath.Join(t.TempDir(), "lab1.clab.yml")
	writeFile(t, topo, "name: lab1\n")

	b, rt := newRuntimeBackend(t)
	containers := labContainers(topo)

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, filters []*clabtypes.GenericFilter,
		) ([]clabruntime.GenericContainer, error) {
			want := []*clabtypes.GenericFilter{
				{FilterType: "label", Field: clabconstants.Containerlab, Operator: "exists"},
			}
			if diff := cmp.Diff(want, filters); diff != "" {
				t.Errorf("unexpected list filters (-want +got):\n%s", diff)
			}

			return containers, nil
		}).AnyTimes()

	link := "https://sshx.io/s/AbCdEf0123#readKey,writeKey"

	rt.EXPECT().Exec(gomock.Any(), "clab-lab1-sshx", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			if diff := cmp.Diff([]string{"cat", "/tmp/sshx"}, cmd.GetCmd()); diff != "" {
				t.Errorf("unexpected command (-want +got):\n%s", diff)
			}

			result := clabexec.NewExecResult(cmd)
			result.SetStdOut([]byte(link + "\n"))

			return result, nil
		}).AnyTimes()

	if err := b.reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicLabs) ||
		!topics.Has(clabinternaltuibackend.TopicLinks) {
		t.Errorf("the first inventory signaled %b", topics)
	}

	// The link of the sshx session is read in the background, which asks for a reload.
	eventually(
		t,
		"the sshx link to be read",
		func() bool { return b.shares.link("sshx-container-id").full != "" },
	)

	if err := b.reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	ref := func(lab, node string) clabinternaltuibackend.NodeRef {
		return clabinternaltuibackend.NodeRef{
			Lab:       lab,
			Name:      node,
			Container: "clab-" + lab + "-" + node,
		}
	}

	want := []clabinternaltuibackend.Lab{
		{
			Name: "lab0", TopoFile: "/gone/lab0.clab.yml", Deployed: true, TopoMissing: true,
			Nodes: []clabinternaltuibackend.Node{{
				NodeRef: ref("lab0", "r1"), ID: "0123456789ab", Kind: "nokia_srlinux",
				Image: "ghcr.io/nokia/srlinux:latest", State: "exited",
				Status: "Exited (137) 2 days ago", Labels: containers[3].Labels,
			}},
		},
		{
			Name: "lab1", TopoFile: topo, LabDir: filepath.Join(filepath.Dir(topo), "clab-lab1"),
			Owner: "alice", Deployed: true,
			Nodes: []clabinternaltuibackend.Node{
				{
					NodeRef: ref("lab1", "leaf2"), ID: "0123456789ab", Kind: "nokia_srlinux",
					Image: "ghcr.io/nokia/srlinux:latest", State: "running",
					Status: "Up 3 hours (unhealthy)", IPv4: "172.20.20.2",
					Ports: []clabinternaltuibackend.Port{{ContainerPort: 830, Protocol: "tcp"}},
					Pid:   4300, Labels: containers[4].Labels,
				},
				{
					NodeRef: ref("lab1", "leaf10"), ID: "0123456789ab", Kind: "nokia_srlinux",
					Type: "ixr-d2l", Group: "leaves", Image: "ghcr.io/nokia/srlinux:latest",
					State: "running", Status: "Up 3 hours (healthy)",
					IPv4: "172.20.20.10", IPv6: "3fff:172:20:20::a",
					Ports: []clabinternaltuibackend.Port{
						{ContainerPort: 22, Protocol: "tcp"},
						{HostIP: "0.0.0.0", HostPort: 50057, ContainerPort: 57400, Protocol: "tcp"},
						{HostIP: "::", HostPort: 50057, ContainerPort: 57400, Protocol: "tcp"},
					},
					Owner: "alice", Pid: 4242, Labels: containers[0].Labels,
				},
			},
			Shares: []clabinternaltuibackend.Share{
				{
					Tool: clabinternaltuibackend.ShareGoTTY, Lab: "lab1",
					Link: "http://" + b.hostname + ":8080",
				},
				{
					Tool: clabinternaltuibackend.ShareSSHX, Lab: "lab1", Link: link,
					ReadOnlyLink: "https://sshx.io/s/AbCdEf0123#readKey",
				},
			},
		},
	}

	got := b.Labs()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected labs (-want +got):\n%s", diff)
	}

	states := []clabinternaltuibackend.LabState{got[0].State(), got[1].State()}
	wantStates := []clabinternaltuibackend.LabState{
		clabinternaltuibackend.LabFailed, clabinternaltuibackend.LabPartial,
	}

	if diff := cmp.Diff(wantStates, states); diff != "" {
		t.Errorf("unexpected lab states (-want +got):\n%s", diff)
	}

	// What is handed out is a copy.
	got[1].Nodes[0].Labels["changed"] = "yes"
	got[1].Nodes[1].Ports[0].ContainerPort = 1

	if diff := cmp.Diff(want, b.Labs()); diff != "" {
		t.Errorf("a change of the returned labs reached the backend (-want +got):\n%s", diff)
	}

	// Only what runs is sampled.
	targets := b.inv.running("")
	if len(targets) != 2 || targets[0].container != "clab-lab1-leaf2" || targets[0].pid != 4300 {
		t.Errorf("unexpected running containers: %+v", targets)
	}

	// Nothing changed: a reload signals nothing.
	b.drainTopics()

	if err := b.reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	if topics := b.drainTopics(); topics.Has(clabinternaltuibackend.TopicLabs) {
		t.Errorf("a reload without changes signaled %b", topics)
	}

	// A node stops: its samples and interface state go with it.
	b.stats.nodes["clab-lab1-leaf2"] = &nodeSample{}
	b.ifaces.containers = map[string]map[string]ifaceState{"clab-lab1-leaf2": {"e1-1": {}}}
	containers[4].State = "exited"
	containers[4].Status = "Exited (0) 1 second ago"

	if err := b.reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	topics := b.drainTopics()
	if !topics.Has(clabinternaltuibackend.TopicLabs) ||
		!topics.Has(clabinternaltuibackend.TopicInterfaces) ||
		topics.Has(clabinternaltuibackend.TopicLinks) {
		t.Errorf("a stopped node signaled %b", topics)
	}

	if _, kept := b.stats.nodes["clab-lab1-leaf2"]; kept ||
		len(b.ifaces.of("clab-lab1-leaf2")) != 0 {
		t.Error("the samples or the interface state of a stopped node were kept")
	}
}

func TestInventoryFailure(t *testing.T) {
	b, rt := newRuntimeBackend(t)

	rt.EXPECT().ListContainers(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("cannot connect to the Docker daemon"))

	err := b.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot connect to the Docker daemon") {
		t.Errorf("Start() = %v, want the runtime error", err)
	}
}

func TestParseSSHXLink(t *testing.T) {
	tests := map[string]struct {
		content string
		want    sshxLink
		ok      bool
	}{
		"writers only": {
			content: "https://sshx.io/s/AbC#key\n",
			want:    sshxLink{full: "https://sshx.io/s/AbC#key"},
			ok:      true,
		},
		"with readers": {
			content: "  https://sshx.io/s/AbC#read,write  \n",
			want: sshxLink{
				full: "https://sshx.io/s/AbC#read,write", readOnly: "https://sshx.io/s/AbC#read",
			},
			ok: true,
		},
		"among other output": {
			content: "sshx v0.4.1\nlink: https://sshx.io/s/AbC#key\n",
			want:    sshxLink{full: "https://sshx.io/s/AbC#key"},
			ok:      true,
		},
		"not ready":            {content: ""},
		"another site":         {content: "https://example.com/s/AbC#key"},
		"with escape sequence": {content: "https://sshx.io/s/\x1b[31mAbC#key"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := parseSSHXLink(tt.content)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%+v, %v), want (%+v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestShareLinkIsNotReadTwice(t *testing.T) {
	var links shareLinks

	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	if !links.claim("c1", now, now.Add(time.Minute)) {
		t.Fatal("a container without a link could not be claimed")
	}

	if links.claim("c1", now.Add(time.Second), now.Add(time.Minute)) {
		t.Error("a container was claimed while its link is being read")
	}

	// After the reader gave up the container is left alone for a while.
	links.giveUp("c1", now.Add(time.Hour))

	if links.claim("c1", now.Add(30*time.Minute), now.Add(time.Hour)) {
		t.Error("a container was claimed right after its link could not be read")
	}

	if !links.claim("c1", now.Add(2*time.Hour), now.Add(3*time.Hour)) {
		t.Error("a container could not be claimed again later")
	}

	links.store("c1", sshxLink{full: "https://sshx.io/s/x#k"})

	if links.claim("c1", now.Add(4*time.Hour), now.Add(5*time.Hour)) {
		t.Error("a container was claimed although its link is known")
	}

	links.retain(map[string]struct{}{})

	if links.link("c1").full != "" {
		t.Error("the link of a container that is gone was kept")
	}
}

func TestInspect(t *testing.T) {
	b := newTestBackend(t)

	containers := fabricContainers(t)
	containers[0].NetworkName = "clab"
	containers[0].NetworkSettings = clabruntime.GenericMgmtIPs{
		IPv4addr: "172.20.20.2", IPv4pLen: 24, IPv4Gw: "172.20.20.1",
	}
	containers[0].Ports = []*clabtypes.GenericPortBinding{
		{HostIP: "::", HostPort: 50057, ContainerPort: 57400, Protocol: "tcp"},
		{ContainerPort: 22, Protocol: "tcp"},
	}
	containers[0].Labels["note"] = "first\nsecond \x1b[31mline\x1b[0m"
	b.setLabs(containers...)

	got, err := b.Inspect(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"})
	if err != nil {
		t.Fatal(err)
	}

	topo := absTestdata(t, "fabric.clab.yml")
	labDir := absTestdata(t, "lab-fabric")

	want := `Node
  name:            leaf1
  lab:             fabric
  kind:            nokia_srlinux

Container
  name:            clab-fabric-leaf1
  id:              0123456789ab
  image:           ghcr.io/nokia/srlinux:latest
  state:           running
  status:          Up 5 minutes
  pid:             4242

Management network
  network:         clab
  ipv4:            172.20.20.2/24
  ipv4 gateway:    172.20.20.1

Ports
  22/tcp
  [::]:50057 -> 57400/tcp

Files
  topology file:   ` + topo + `
  lab directory:   ` + labDir + `
  node directory:  ` + labDir + `/leaf1

Topology
  index:           1
  long name:       clab-fabric-leaf1
  fqdn:            leaf1.fabric.io
  mgmt network:    clab
  mgmt interface:  mgmt0
  mgmt ipv4:       172.20.20.2/24
  mgmt ipv6:       3fff:172:20:20::2/64
  mac address:     1a:b5:00:00:00:00

Links
  e1-1 <-> spine1:e1-1
  ethernet-1/2 <-> client1:eth1
  e1-3 <-> host:leaf1-e1-3

Labels
  clab-node-kind=nokia_srlinux
  clab-node-lab-dir=` + labDir + `/leaf1
  clab-node-longname=clab-fabric-leaf1
  clab-node-name=leaf1
  clab-topo-file=` + topo + `
  containerlab=fabric
  note=first second line
`

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected description (-want +got):\n%s", diff)
	}

	// A node that only has one end of a link, and port bindings in the export.
	client, err := b.Inspect(context.Background(),
		clabinternaltuibackend.NodeRef{Container: "clab-fabric-client1"})
	if err != nil {
		t.Fatal(err)
	}

	for _, part := range []string{
		"  port binding:    0.0.0.0:8080 -> 80/tcp\n",
		"Links\n  eth1 <-> leaf1:ethernet-1/2\n  eth2\n",
	} {
		if !strings.Contains(client, part) {
			t.Errorf("the description lacks %q:\n%s", part, client)
		}
	}

	if _, err := b.Inspect(context.Background(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "nope"}); err == nil {
		t.Error("inspecting an unknown node did not fail")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := b.Inspect(
		ctx,
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"},
	); err == nil {
		t.Error("inspecting with a context that ended did not fail")
	}
}

func TestInterfacesOfNode(t *testing.T) {
	b := newTestBackend(t)
	b.setLabs(fabricContainers(t)...)

	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}

	if got := b.Interfaces(leaf1); len(got) != 0 {
		t.Errorf("interfaces before anything was sampled: %+v", got)
	}

	// The node's operating system names its interfaces differently than Linux does. The
	// event stream reports the Linux name with the other one as alias.
	c := newFakeContainer(t, b.stats.root)
	c.setNetDev(
		netDevLine("mgmt0", 100, 1, 0, 0),
		netDevLine("e1-1", 1000, 10, 0, 0),
		netDevLine(
			"e1-2",
			0,
			0,
			0,
			0,
		),
		netDevLine("e1-3", 0, 0, 0, 0),
		netDevLine("e1-10", 0, 0, 0, 0),
	)
	b.stats.sample(b.clock.Now(), []sampleTarget{{container: "clab-fabric-leaf1", pid: c.pid}})

	for name, alias := range map[string]string{"e1-1": "ethernet-1/1", "e1-2": "ethernet-1/2"} {
		b.ifaces.apply(&streamEvent{
			Type: eventTypeInterface, Action: "snapshot", ActorName: "clab-fabric-leaf1",
			Attributes: map[string]string{"ifname": name, "alias": alias, "state": "up"},
		})
	}

	got := b.Interfaces(leaf1)

	type row struct {
		Name, Alias, State string
		Peer               *clabinternaltuibackend.Endpoint
		RxBytes            uint64
	}

	rows := make([]row, 0, len(got))

	for _, iface := range got {
		if iface.Node.Container != "clab-fabric-leaf1" || iface.Node.Name != "leaf1" {
			t.Errorf("interface %s belongs to %+v", iface.Name, iface.Node)
		}

		rows = append(rows, row{iface.Name, iface.Alias, iface.State, iface.Peer, iface.RxBytes})
	}

	want := []row{
		{
			Name: "e1-1", Alias: "ethernet-1/1", State: stateUp, RxBytes: 1000,
			Peer: &clabinternaltuibackend.Endpoint{
				Node: "spine1", Interface: "e1-1", MAC: "aa:c1:ab:5b:85:30",
			},
		},
		// The topology names this interface the way the node does.
		{
			Name: "e1-2", Alias: "ethernet-1/2", State: stateUp,
			Peer: &clabinternaltuibackend.Endpoint{
				Node: "client1", Interface: "eth1", MAC: "aa:c1:ab:00:00:03",
			},
		},
		{
			Name: "e1-3",
			Peer: &clabinternaltuibackend.Endpoint{
				Node: "host", Interface: "leaf1-e1-3", MAC: "aa:c1:ab:00:00:06", External: true,
			},
		},
		{Name: "e1-10"},
		{Name: "mgmt0", RxBytes: 100},
	}

	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("unexpected interfaces (-want +got):\n%s", diff)
	}

	// The peer handed out is a copy.
	got[0].Peer.Node = "changed"

	if again := b.Interfaces(leaf1); again[0].Peer.Node != "spine1" {
		t.Error("a change of a returned peer reached the backend")
	}

	if got := b.Interfaces(
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "nope"},
	); got != nil {
		t.Errorf("interfaces of an unknown node: %+v", got)
	}
}

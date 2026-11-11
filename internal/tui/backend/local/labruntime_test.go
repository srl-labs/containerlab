// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabcore "github.com/srl-labs/containerlab/core"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const testLabRuntime = "c9s"

// fakeLabRuntime is a lab runtime whose labs and answers a test sets.
type fakeLabRuntime struct {
	mu   sync.Mutex
	labs []*clablabruntime.LabState

	stats    map[string][]clablabruntime.NodeStats
	statsErr error

	interfaces    []clablabruntime.CaptureInterface
	interfacesErr error

	logs    string
	logReqs []clablabruntime.LogRequest

	execReqs []clablabruntime.ExecRequest

	events     chan clablabruntime.Event
	streamErrs chan error
	streamReqs []clablabruntime.EventStreamRequest

	attachReqs []clablabruntime.AttachRequest
	dialed     []int
	remote     net.Conn
}

var (
	_ clablabruntime.LabRuntime        = (*fakeLabRuntime)(nil)
	_ clablabruntime.NodeLogStreamer   = (*fakeLabRuntime)(nil)
	_ clablabruntime.NodeStatsSampler  = (*fakeLabRuntime)(nil)
	_ clablabruntime.PacketCapturer    = (*fakeLabRuntime)(nil)
	_ clablabruntime.NodeAttacher      = (*fakeLabRuntime)(nil)
	_ clablabruntime.NodePortForwarder = (*fakeLabRuntime)(nil)
)

func (*fakeLabRuntime) Deploy(
	context.Context, clablabruntime.DeployRequest,
) (*clablabruntime.LabState, error) {
	return nil, errors.New("not used")
}

func (*fakeLabRuntime) Destroy(context.Context, clablabruntime.DestroyRequest) error {
	return errors.New("not used")
}

func (r *fakeLabRuntime) Inspect(
	_ context.Context, req clablabruntime.InspectRequest,
) (*clablabruntime.LabState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, lab := range r.labs {
		if lab.Name == req.Name && (req.Namespace == "" || req.Namespace == lab.Namespace) {
			return lab, nil
		}
	}

	return nil, errors.New("lab not found")
}

func (r *fakeLabRuntime) List(
	context.Context, clablabruntime.ListRequest,
) ([]*clablabruntime.LabState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.labs), nil
}

func (r *fakeLabRuntime) Exec(
	_ context.Context, req clablabruntime.ExecRequest,
) (*clabexec.ExecResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.execReqs = append(r.execReqs, req)

	result := clabexec.NewExecResult(clabexec.NewExecCmdFromSlice(req.Command))
	result.SetStdOut([]byte("output of " + req.NodeName))

	return result, nil
}

func (*fakeLabRuntime) Start(context.Context, clablabruntime.NodeRequest) error   { return nil }
func (*fakeLabRuntime) Stop(context.Context, clablabruntime.NodeRequest) error    { return nil }
func (*fakeLabRuntime) Restart(context.Context, clablabruntime.NodeRequest) error { return nil }

func (*fakeLabRuntime) Save(
	context.Context, clablabruntime.SaveRequest,
) (*clablabruntime.SaveResult, error) {
	return &clablabruntime.SaveResult{}, nil
}

func (r *fakeLabRuntime) StreamEvents(
	_ context.Context, req clablabruntime.EventStreamRequest,
) (<-chan clablabruntime.Event, <-chan error, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.streamReqs = append(r.streamReqs, req)

	return r.events, r.streamErrs, nil
}

func (r *fakeLabRuntime) StreamLogs(
	_ context.Context, req clablabruntime.LogRequest, w io.Writer,
) error {
	r.mu.Lock()
	r.logReqs = append(r.logReqs, req)
	logs := r.logs
	r.mu.Unlock()

	_, err := io.WriteString(w, logs)

	return err
}

func (r *fakeLabRuntime) SampleStats(
	_ context.Context, req clablabruntime.InspectRequest,
) ([]clablabruntime.NodeStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.statsErr != nil {
		return nil, r.statsErr
	}

	return r.stats[req.Namespace+"/"+req.Name], nil
}

func (r *fakeLabRuntime) CaptureInterfaces(
	context.Context, clablabruntime.NodeRequest,
) ([]clablabruntime.CaptureInterface, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.interfaces, r.interfacesErr
}

func (*fakeLabRuntime) Capture(context.Context, clablabruntime.CaptureRequest, io.Writer) error {
	return errors.New("not used")
}

// Attach answers what it reads with a greeting, the way a shell echoes its input.
func (r *fakeLabRuntime) Attach(
	_ context.Context, req clablabruntime.AttachRequest, streams clablabruntime.AttachStreams,
) error {
	r.mu.Lock()
	r.attachReqs = append(r.attachReqs, req)
	r.mu.Unlock()

	input, err := io.ReadAll(streams.Stdin)
	if err != nil {
		return err
	}

	_, err = io.WriteString(streams.Stdout, "hello "+string(input))

	return err
}

func (r *fakeLabRuntime) DialPort(
	_ context.Context, _ clablabruntime.NodeRequest, port int,
) (net.Conn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.dialed = append(r.dialed, port)

	if r.remote == nil {
		return nil, errors.New("no such port")
	}

	return r.remote, nil
}

// labNode returns a node as a lab runtime reports it.
func labNode(name, kind, state string) clablabruntime.NodeState {
	node := clablabruntime.NodeState{
		Name:            name,
		Kind:            kind,
		Image:           "ghcr.io/nokia/srlinux:latest",
		ContainerState:  state,
		MgmtIPv4Address: "172.20.20.2/24",
		Pod:             name + "-7cfd5475fd-gq5b5",
		Container:       "node-" + name,
		NodeUID:         "uid-" + name,
	}

	if state == clablabruntime.ContainerStateRunning {
		node.Health = clablabruntime.ContainerHealthHealthy
	}

	return node
}

// fabricLabState is the lab the tests of this file work on: two running nodes, one of
// which the cluster carries under another name than the topology, and a stopped one.
func fabricLabState(topoPath string) *clablabruntime.LabState {
	renamed := labNode("r1", "nokia_srlinux", clablabruntime.ContainerStateRunning)
	renamed.TopologyName = "R1"

	return &clablabruntime.LabState{
		Name:         "fabric",
		Namespace:    "c9s-fabric",
		Owner:        "alice",
		TopologyPath: topoPath,
		Nodes: []clablabruntime.NodeState{
			labNode("leaf1", "nokia_srlinux", clablabruntime.ContainerStateRunning),
			renamed,
			labNode("client1", "linux", clablabruntime.ContainerStateExited),
		},
	}
}

// newLabRuntimeBackend returns a backend for the labs of a fake lab runtime, with the
// inventory loaded.
func newLabRuntimeBackend(
	t *testing.T,
	rt *fakeLabRuntime,
	adjust ...func(o *Options, d *deps),
) *testBackend {
	t.Helper()

	c, err := clabcore.NewContainerLab(
		clabcore.WithRuntime(testLabRuntime, &clabruntime.RuntimeConfig{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	c.LabRuntime = rt

	b := newTestBackend(t, append([]func(o *Options, d *deps){
		func(_ *Options, d *deps) {
			d.clab, d.lrt, d.rtName = c, rt, testLabRuntime
		},
	}, adjust...)...)

	if err := b.reload(t.Context()); err != nil {
		t.Fatal(err)
	}

	return b
}

func TestLabRuntimeInventory(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{
		fabricLabState("k8s://c9s-fabric/topologies/fabric"),
	}}
	b := newLabRuntimeBackend(t, rt)

	host := b.Host()
	if !host.Cluster || !host.Privileged || host.Runtime != testLabRuntime {
		t.Errorf("unexpected host: %+v", host)
	}

	labs := b.Labs()
	if len(labs) != 1 {
		t.Fatalf("got %d labs, want one: %+v", len(labs), labs)
	}

	lab := labs[0]
	if lab.Name != "fabric" || lab.Namespace != "c9s-fabric" || !lab.Deployed ||
		lab.Owner != "alice" {
		t.Errorf("unexpected lab: %+v", lab)
	}

	// The place the cluster keeps the definition at is not a file.
	if lab.TopoFile != "" || !lab.TopoMissing {
		t.Errorf("topology file %q, missing %v", lab.TopoFile, lab.TopoMissing)
	}

	names := make([]string, 0, len(lab.Nodes))
	for i := range lab.Nodes {
		names = append(names, lab.Nodes[i].Name+"="+lab.Nodes[i].Readiness().String())
	}

	// The nodes go by the names of the topology, in the order the UI lists them in.
	want := []string{"client1=stopped", "leaf1=ready", "R1=ready"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("unexpected nodes (-want +got):\n%s", diff)
	}

	rec, node, ctr := b.inv.find(clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "R1"})
	if node == nil || ctr == nil {
		t.Fatal("the renamed node was not found")
	}

	if node.Container != "fabric-r1" || rec.runtimeNode(node) != "r1" ||
		rec.stateKey(node.Container) != "c9s-fabric/fabric/r1" {
		t.Errorf("container %q, runtime name %q, state key %q",
			node.Container, rec.runtimeNode(node), rec.stateKey(node.Container))
	}

	if got := b.inv.runningKeys(); len(got) != 2 {
		t.Errorf("running keys: %v", got)
	}

	// Nothing of a lab in a cluster is read from this host.
	if targets := b.inv.running(""); len(targets) != 0 {
		t.Errorf("nodes of the cluster are sampled on this host: %+v", targets)
	}
}

func TestLabsOfTheSameNameAreToldApart(t *testing.T) {
	shared := fabricLabState("")
	shared.Namespace = "shared"

	other := &clablabruntime.LabState{
		Name: "edge", Namespace: "c9s-edge",
		Nodes: []clablabruntime.NodeState{
			labNode("pe1", "linux", clablabruntime.ContainerStateRunning),
		},
	}

	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState(""), shared, other}}
	b := newLabRuntimeBackend(t, rt)

	var names []string
	for _, lab := range b.Labs() {
		if lab.Deployed {
			names = append(names, lab.Name)
		}
	}

	want := []string{"edge", "fabric@c9s-fabric", "fabric@shared"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Fatalf("unexpected labs (-want +got):\n%s", diff)
	}

	ref := clabinternaltuibackend.NodeRef{Lab: "fabric@shared", Name: "leaf1"}

	rec, node, ctr := b.inv.find(ref)
	if node == nil || ctr == nil {
		t.Fatal("the node of the renamed lab was not found")
	}

	if rec.name != "fabric" || rec.namespace != "shared" || node.Lab != "fabric@shared" ||
		node.Container != "fabric@shared-leaf1" ||
		rec.stateKey(node.Container) != "shared/fabric/leaf1" {
		t.Errorf("lab %q in %q, node of %q in container %q", rec.name, rec.namespace,
			node.Lab, node.Container)
	}

	// The container names tell the nodes apart too.
	if _, byContainer, _ := b.inv.find(
		clabinternaltuibackend.NodeRef{Container: "fabric@c9s-fabric-leaf1"},
	); byContainer == nil || byContainer.Lab != "fabric@c9s-fabric" {
		t.Errorf("found %+v by the container name", byContainer)
	}
}

// interfaceEvent returns an interface event of a node of the fabric lab.
func interfaceEvent(
	node, action string,
	iface clablabruntime.InterfaceEventData,
) *clablabruntime.Event {
	return &clablabruntime.Event{
		Type:        clablabruntime.EventTypeInterface,
		Action:      action,
		ActorName:   "fabric-" + node,
		ActorFullID: "c9s-fabric/fabric/" + node,
		Interface:   &iface,
	}
}

func TestLabRuntimeEvents(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState("")}}
	b := newLabRuntimeBackend(t, rt)
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}

	b.drainTopics()
	b.applyLabRuntimeEvent(interfaceEvent("leaf1", "snapshot", clablabruntime.InterfaceEventData{
		Name: "e1-1", Alias: "ethernet-1/1", Index: 7, MTU: 9500,
		MAC: "aa:c1:ab:00:00:01", OperState: "up", Type: "veth",
	}))
	b.applyLabRuntimeEvent(interfaceEvent("leaf1", "snapshot", clablabruntime.InterfaceEventData{
		Name: "lo", OperState: "unknown", Type: "device",
	}))

	if !b.drainTopics().Has(clabinternaltuibackend.TopicInterfaces) {
		t.Error("a new interface was not signaled")
	}

	ifaces := b.Interfaces(leaf1)
	if len(ifaces) != 1 {
		t.Fatalf("got %d interfaces, want the link interface only: %+v", len(ifaces), ifaces)
	}

	if got := ifaces[0]; got.Name != "e1-1" || got.Alias != "ethernet-1/1" ||
		got.State != stateUp || got.MTU != 9500 || got.MAC != "aa:c1:ab:00:00:01" ||
		got.HasStats {
		t.Errorf("unexpected interface: %+v", got)
	}

	for _, name := range []string{"e1-1", "lo"} {
		b.applyLabRuntimeEvent(interfaceEvent("leaf1", ifaceActionStats,
			clablabruntime.InterfaceEventData{
				Name: name,
				Stats: &clablabruntime.InterfaceStats{
					RxBytes: 1000, TxBytes: 2000, RxPackets: 10, TxPackets: 20,
					RxBps: 800, TxBps: 1600, RxPps: 1, TxPps: 2,
				},
			}))
	}

	ifaces = b.Interfaces(leaf1)
	if got := ifaces[0]; len(ifaces) != 1 || !got.HasStats || got.RxBps != 800 ||
		got.TxBps != 1600 || got.RxBytes != 1000 || got.TxPackets != 20 ||
		len(got.RxSeries) != 1 {
		t.Errorf("unexpected counters: %+v", ifaces)
	}

	// The traffic of the node is summed up per reading of the runtime, the loopback
	// interface left out.
	if !b.stats.rollup() || b.stats.rollup() {
		t.Error("the counters were not summed up once")
	}

	if node := b.Labs()[0].Node("leaf1"); node.RxBps != 800 || node.TxBps != 1600 ||
		len(node.RxSeries) != 1 {
		t.Errorf("unexpected traffic of the node: %+v", node)
	}

	b.applyLabRuntimeEvent(interfaceEvent("leaf1", ifaceActionDelete,
		clablabruntime.InterfaceEventData{Name: "e1-1"}))

	if ifaces := b.Interfaces(leaf1); len(ifaces) != 0 {
		t.Errorf("a deleted interface is still listed: %+v", ifaces)
	}

	// Any other event stands for a change of the inventory.
	select {
	case <-b.reloadCh:
	default:
	}

	b.applyLabRuntimeEvent(&clablabruntime.Event{
		Type: clablabruntime.EventTypeContainer, Action: "die",
		ActorFullID: "c9s-fabric/fabric/leaf1",
	})

	select {
	case <-b.reloadCh:
	default:
		t.Error("a container event did not ask for a reload")
	}
}

func TestLabRuntimeStream(t *testing.T) {
	rt := &fakeLabRuntime{
		labs:       []*clablabruntime.LabState{fabricLabState("")},
		events:     make(chan clablabruntime.Event),
		streamErrs: make(chan error),
	}
	b := newLabRuntimeBackend(t, rt)
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}

	if caps := b.Host().Caps; caps.LinkState {
		t.Errorf("the state of the links is known before the stream runs: %+v", caps)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		b.streamLabRuntime(ctx)
	}()

	rt.events <- *interfaceEvent("leaf1", "create",
		clablabruntime.InterfaceEventData{Name: "e1-1", OperState: "down"})

	eventually(t, "the interface to be listed", func() bool {
		ifaces := b.Interfaces(leaf1)

		return len(ifaces) == 1 && ifaces[0].State == stateDown
	})

	if !b.Host().Caps.LinkState {
		t.Error("the state of the links is not known while the stream runs")
	}

	rt.mu.Lock()
	req := rt.streamReqs[0]
	rt.mu.Unlock()

	if !req.AllNamespaces || !req.IncludeInitialState || !req.IncludeInterfaceStats ||
		req.StatsInterval != labRuntimeStatsInterval {
		t.Errorf("unexpected stream request: %+v", req)
	}

	// What a stream told is forgotten when it fails: a new one reports all of it again.
	rt.streamErrs <- errors.New("watch closed")

	eventually(t, "the interfaces to be forgotten", func() bool {
		return len(b.Interfaces(leaf1)) == 0 && !b.Host().Caps.LinkState
	})

	caps := b.Host().Caps
	if reason := caps.Reason(capLinkState); !strings.Contains(reason, "watch closed") {
		t.Errorf("unexpected reason: %q", reason)
	}

	cancel()
	<-done
}

func TestLabRuntimeUsage(t *testing.T) {
	rt := &fakeLabRuntime{
		labs: []*clablabruntime.LabState{fabricLabState("")},
		stats: map[string][]clablabruntime.NodeStats{
			"c9s-fabric/fabric": {
				{NodeName: "leaf1", CPUMilliCores: 250, MemoryBytes: 64 << 20},
				{NodeName: "r1", CPUMilliCores: 1500, MemoryBytes: 1 << 30},
			},
		},
	}
	b := newLabRuntimeBackend(t, rt)

	b.drainTopics()

	if err := b.sampleUsage(t.Context(), rt); err != nil {
		t.Fatal(err)
	}

	if !b.drainTopics().Has(clabinternaltuibackend.TopicStats) {
		t.Error("new usage values were not signaled")
	}

	lab := b.Labs()[0]

	if node := lab.Node("leaf1"); !node.HasStats || node.CPUPercent != 25 ||
		node.MemBytes != 64<<20 || len(node.CPUSeries) != 1 {
		t.Errorf("unexpected usage of leaf1: %+v", node)
	}

	// Usage is reported under the name the cluster knows the node by.
	if node := lab.Node("R1"); !node.HasStats || node.CPUPercent != 150 {
		t.Errorf("unexpected usage of R1: %+v", node)
	}

	if node := lab.Node("client1"); node.HasStats {
		t.Errorf("a stopped node has usage values: %+v", node)
	}

	rt.mu.Lock()
	rt.statsErr = errors.Join(clablabruntime.ErrMetricsUnavailable, errors.New("no metrics"))
	rt.mu.Unlock()

	if err := b.sampleUsage(t.Context(), rt); !errors.Is(err,
		clablabruntime.ErrMetricsUnavailable) {
		t.Errorf("err = %v, want the metrics to be unavailable", err)
	}
}

// runArgs runs an operation and returns the arguments of the command it started, which is
// made to succeed.
func runArgs(
	t *testing.T,
	b *testBackend,
	op func() (clabinternaltuibackend.JobID, error),
) string {
	t.Helper()

	started := b.run.count()

	id, err := op()
	if err != nil {
		t.Fatal(err)
	}

	proc := b.run.proc(t, started)
	proc.exitWith(0)
	b.awaitJob(t, id)

	return strings.Join(proc.spec.args, " ")
}

func TestLabRuntimeCommands(t *testing.T) {
	shared := &clablabruntime.LabState{
		Name: "edge", Namespace: "shared",
		Nodes: []clablabruntime.NodeState{
			labNode("pe1", "linux", clablabruntime.ContainerStateRunning),
		},
	}
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState(""), shared}}
	b := newLabRuntimeBackend(t, rt)

	topo := absTestdata(t, "fabric.clab.yml")
	capture := filepath.Join(t.TempDir(), "x.pcapng")

	tests := map[string]struct {
		op   func() (clabinternaltuibackend.JobID, error)
		want string
	}{
		// The topology file of the lab is not needed to find its nodes.
		"stop": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "fabric", Action: clabinternaltuibackend.NodeStop,
					Nodes: []string{"R1"},
				})
			},
			want: "stop -r c9s --name fabric --node R1",
		},
		// A lab outside of the namespace its name leads to is found by its namespace.
		"start in another namespace": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.NodeAction(clabinternaltuibackend.NodeActionRequest{
					Lab: "edge", Action: clabinternaltuibackend.NodeStart,
				})
			},
			want: "start -r c9s --name edge --namespace shared",
		},
		"save": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.Save(clabinternaltuibackend.SaveRequest{Lab: "fabric"})
			},
			want: "save -r c9s --name fabric",
		},
		"deploy": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.Deploy(clabinternaltuibackend.DeployRequest{TopoFile: topo})
			},
			want: "deploy -r c9s -t " + topo + " --format json",
		},
		"destroy in another namespace": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "edge", Cleanup: true})
			},
			want: "destroy -r c9s --name edge --cleanup --namespace shared",
		},
		// The lab is named to the capture command, which asks the runtime for its links.
		"capture": {
			op: func() (clabinternaltuibackend.JobID, error) {
				return b.Capture(clabinternaltuibackend.CaptureRequest{
					Lab: "fabric", File: capture,
					Targets: []clabinternaltuibackend.CaptureTarget{
						{Node: "leaf1", Interfaces: []string{"e1-1", "e1-2"}},
					},
				})
			},
			want: "capture -r c9s --name fabric --target leaf1:e1-1,e1-2 -w " + capture,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := runArgs(t, b, tt.op); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}

	if _, err := b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab: "nope", File: capture,
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf1", Interfaces: []string{"e1-1"}},
		},
	}); err == nil || !strings.Contains(err.Error(), "not deployed") {
		t.Errorf("a capture in an unknown lab: err = %v", err)
	}
}

func TestLabRuntimeCommandsInTheNamespaceAskedFor(t *testing.T) {
	lab := fabricLabState("")
	lab.Namespace = "team"

	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{lab}}
	b := newLabRuntimeBackend(t, rt, func(o *Options, _ *deps) { o.Namespace = "team" })

	topo := absTestdata(t, "fabric.clab.yml")

	// A lab is deployed to the namespace the UI was started for.
	got := runArgs(t, b, func() (clabinternaltuibackend.JobID, error) {
		return b.Redeploy(clabinternaltuibackend.RedeployRequest{Lab: "fabric", TopoFile: topo})
	})
	if want := "redeploy -r c9s -t " + topo + " --format json --namespace team"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	got = runArgs(t, b, func() (clabinternaltuibackend.JobID, error) {
		return b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "fabric"})
	})
	if want := "destroy -r c9s --name fabric --namespace team"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestLabRuntimeLeavesTheHostAlone(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState("")}}
	b := newLabRuntimeBackend(t, rt)
	leaf1 := clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}

	_, netem := b.SetNetem(clabinternaltuibackend.NetemRequest{
		Interfaces: interfaceRefs(leaf1, "e1-1"),
	})
	_, reset := b.ResetNetem(interfaceRefs(leaf1, "e1-1"))
	_, share := b.Share(clabinternaltuibackend.ShareRequest{
		Lab: "fabric", Tool: clabinternaltuibackend.ShareSSHX,
		Action: clabinternaltuibackend.ShareAttach,
	})

	for op, err := range map[string]error{"netem": netem, "netem reset": reset, "share": share} {
		if !errors.Is(err, clabinternaltuibackend.ErrUnavailable) ||
			!strings.Contains(err.Error(), "c9s runtime") {
			t.Errorf("%s: err = %v, want it to be unavailable with the runtime", op, err)
		}
	}

	if b.run.count() != 0 {
		t.Error("a command was run")
	}

	caps := b.Host().Caps
	if !caps.Lifecycle || !caps.Capture || caps.Netem || caps.Share {
		t.Errorf("unexpected capabilities: %+v", caps)
	}

	if !strings.Contains(caps.Reason(capNetem), "impairing links") ||
		!strings.Contains(caps.Reason(capShare), "sharing a terminal") {
		t.Errorf("unexpected reasons: %v", caps.Reasons)
	}

	// The capture command needs no root privileges on the host the user logs in to.
	if program := b.remoteProgram("alice"); strings.HasPrefix(program, "sudo") {
		t.Errorf("the capture is run as %q for a user who logs in", program)
	}
}

func TestLabRuntimeLogs(t *testing.T) {
	rt := &fakeLabRuntime{
		labs: []*clablabruntime.LabState{fabricLabState("")},
		logs: "booting\nWARN low memory\n",
	}
	b := newLabRuntimeBackend(t, rt)

	stream, err := b.Logs(t.Context(),
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "R1"},
		clabinternaltuibackend.LogOptions{Tail: 100, Follow: true})
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	for batch := range stream.Lines() {
		for i := range batch {
			lines = append(lines, batch[i].Text)
		}
	}

	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"booting", "WARN low memory"}, lines); diff != "" {
		t.Errorf("unexpected lines (-want +got):\n%s", diff)
	}

	rt.mu.Lock()
	req := rt.logReqs[0]
	rt.mu.Unlock()

	if req.Name != "fabric" || req.Namespace != "c9s-fabric" || req.NodeName != "r1" ||
		req.Tail == nil || *req.Tail != 100 || !req.Follow {
		t.Errorf("unexpected log request: %+v", req)
	}
}

func TestLabRuntimeExec(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState("")}}
	b := newLabRuntimeBackend(t, rt)

	results, err := b.Exec(t.Context(), []clabinternaltuibackend.NodeRef{
		{Lab: "fabric", Name: "R1"},
	}, "ip -br link")
	if err != nil {
		t.Fatal(err)
	}

	result := <-results
	if result.Err != nil || result.Stdout != "output of r1" || result.Node.Name != "R1" {
		t.Errorf("unexpected result: %+v", result)
	}

	rt.mu.Lock()
	req := rt.execReqs[0]
	rt.mu.Unlock()

	if req.Name != "fabric" || req.Namespace != "c9s-fabric" || req.NodeName != "r1" ||
		!slices.Equal(req.Command, []string{"ip", "-br", "link"}) {
		t.Errorf("unexpected exec request: %+v", req)
	}
}

func TestLinksFromInterfaces(t *testing.T) {
	interfaces := []clablabruntime.CaptureInterface{
		// A link between two nodes is listed from both of its ends.
		{
			Node:          "leaf1",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "spine1",
			PeerInterface: "e1-1",
		},
		{
			Node:          "spine1",
			Interface:     "e1-1",
			Alias:         "ethernet-1/1",
			PeerNode:      "leaf1",
			PeerInterface: "ethernet-1/1",
		},
		{Node: "leaf1", Interface: "e1-2", PeerNode: "host", PeerInterface: "leaf1-e1-2"},
		{Node: "spine1", Interface: "e1-9"},
	}

	got := linksFromInterfaces(interfaces, map[string]bool{"leaf1": true, "spine1": true})

	want := []clabinternaltuibackend.Link{
		{
			A: clabinternaltuibackend.Endpoint{Node: "leaf1", Interface: "e1-2"},
			Z: &clabinternaltuibackend.Endpoint{
				Node: "host", Interface: "leaf1-e1-2", External: true,
			},
		},
		{
			A: clabinternaltuibackend.Endpoint{Node: "leaf1", Interface: "ethernet-1/1"},
			Z: &clabinternaltuibackend.Endpoint{Node: "spine1", Interface: "ethernet-1/1"},
		},
		{A: clabinternaltuibackend.Endpoint{Node: "spine1", Interface: "e1-9"}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected links (-want +got):\n%s", diff)
	}
}

func TestLabRuntimeTopologyWithoutFile(t *testing.T) {
	rt := &fakeLabRuntime{
		labs: []*clablabruntime.LabState{fabricLabState("")},
		interfaces: []clablabruntime.CaptureInterface{
			{Node: "leaf1", Interface: "e1-1", PeerNode: "R1", PeerInterface: "e1-1"},
			{Node: "R1", Interface: "e1-1", PeerNode: "leaf1", PeerInterface: "e1-1"},
		},
	}
	b := newLabRuntimeBackend(t, rt)

	// The links are asked from the runtime in the background.
	if topo := b.Topology("fabric"); len(topo.Nodes) != 3 || topo.Warning == "" {
		t.Errorf("unexpected first topology: %+v", topo)
	}

	var topo clabinternaltuibackend.Topology

	eventually(t, "the links to be read", func() bool {
		topo = b.Topology("fabric")

		return topo.Warning == "" && len(topo.Links) == 1
	})

	if link := topo.Links[0]; link.A.Node != "leaf1" || link.Z == nil || link.Z.Node != "R1" ||
		link.Z.External {
		t.Errorf("unexpected link: %+v", link)
	}

	// The interfaces of the nodes learn their far ends from the same links.
	b.applyLabRuntimeEvent(interfaceEvent("leaf1", "snapshot",
		clablabruntime.InterfaceEventData{Name: "e1-1", OperState: "up"}))

	ifaces := b.Interfaces(clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"})
	if len(ifaces) != 1 || ifaces[0].Peer == nil || ifaces[0].Peer.Node != "R1" {
		t.Errorf("unexpected interfaces: %+v", ifaces)
	}

	// A lab of which nothing runs has no interfaces to ask for.
	rt.mu.Lock()
	for i := range rt.labs[0].Nodes {
		rt.labs[0].Nodes[i].ContainerState = clablabruntime.ContainerStateExited
	}
	rt.mu.Unlock()

	if err := b.reload(t.Context()); err != nil {
		t.Fatal(err)
	}

	if topo := b.Topology("fabric"); !strings.Contains(topo.Warning, "no node") ||
		len(topo.Links) != 0 {
		t.Errorf("unexpected topology of a stopped lab: %+v", topo)
	}
}

func TestLabRuntimeTopologyFromFile(t *testing.T) {
	topoFile := absTestdata(t, "fabric.clab.yml")
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState(topoFile)}}

	parsed := make(chan string, 1)
	b := newLabRuntimeBackend(t, rt, func(_ *Options, d *deps) {
		d.loadTopo = func(file string) (*clabcore.CLab, error) {
			parsed <- file

			return nil, errors.New("not parsed in this test")
		}
	})

	if lab := b.Labs()[0]; lab.TopoFile != topoFile || lab.TopoMissing {
		t.Fatalf("unexpected lab: %+v", lab)
	}

	// With the file at hand the topology is read from it, not asked from the cluster.
	b.Topology("fabric")

	select {
	case file := <-parsed:
		if file != topoFile {
			t.Errorf("parsed %q", file)
		}
	case <-time.After(testTimeout):
		t.Fatal("the topology file was not parsed")
	}
}

func TestLabRuntimeShell(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState("")}}
	b := newLabRuntimeBackend(t, rt)

	session, err := b.Interactive(
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "R1"},
		clabinternaltuibackend.ConnectMethod{Shell: []string{"sr_cli"}},
	)
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder

	// Without a terminal the session passes the streams on as they are.
	session.SetStdin(strings.NewReader("world"))
	session.SetStdout(&out)
	session.SetStderr(io.Discard)

	if err := session.Run(); err != nil {
		t.Fatal(err)
	}

	if out.String() != "hello world" {
		t.Errorf("the session printed %q", out.String())
	}

	rt.mu.Lock()
	req := rt.attachReqs[0]
	rt.mu.Unlock()

	if req.Name != "fabric" || req.Namespace != "c9s-fabric" || req.NodeName != "r1" ||
		!req.TTY || !slices.Equal(req.Command, []string{"sr_cli"}) {
		t.Errorf("unexpected attach request: %+v", req)
	}

	if _, err := b.Interactive(clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "nope"},
		clabinternaltuibackend.ConnectMethod{}); err == nil {
		t.Error("an unknown node was not reported")
	}
}

func TestLabRuntimeSSH(t *testing.T) {
	near, far := net.Pipe()
	rt := &fakeLabRuntime{
		labs:   []*clablabruntime.LabState{fabricLabState("")},
		remote: near,
	}
	b := newLabRuntimeBackend(t, rt)

	methods := b.ConnectMethods(&clabinternaltuibackend.Node{Kind: "nokia_srlinux"})
	if !methods[1].SSH || !strings.Contains(methods[1].Description, "forwarded") {
		t.Errorf("unexpected methods: %+v", methods)
	}

	session, err := b.Interactive(
		clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "leaf1"}, methods[1],
	)
	if err != nil {
		t.Fatal(err)
	}

	forward, ok := session.(*forwardSession)
	if !ok {
		t.Fatalf("the session is a %T", session)
	}

	// The user is the one the kind of the node is logged in to by default.
	if forward.ssh != "/usr/bin/ssh" || forward.user != "admin" || forward.node != "fabric-leaf1" {
		t.Errorf("unexpected session: %+v", forward)
	}

	want := []string{
		"-p", "2222", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "HostKeyAlias=fabric-leaf1", "admin@127.0.0.1",
	}
	if diff := cmp.Diff(want, sshForwardArgs(2222, forward.user, forward.node)); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	// What connects to the forwarded port reaches the SSH server of the node.
	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	forwarded := make(chan error, 1)

	go func() { forwarded <- forward.forward(t.Context(), listener) }()

	var dialer net.Dialer

	client, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Write([]byte("SSH-2.0")); err != nil {
		t.Fatal(err)
	}

	banner := make([]byte, len("SSH-2.0"))
	if _, err := io.ReadFull(far, banner); err != nil || string(banner) != "SSH-2.0" {
		t.Errorf("the node received %q: %v", banner, err)
	}

	client.Close()

	if err := <-forwarded; err != nil {
		t.Error(err)
	}

	rt.mu.Lock()
	dialed := slices.Clone(rt.dialed)
	rt.mu.Unlock()

	if !slices.Equal(dialed, []int{sshPort}) {
		t.Errorf("dialed the ports %v", dialed)
	}

	// The port is open for the one connection of the session only.
	if conn, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String()); err == nil {
		conn.Close()
		t.Error("the forwarded port took a second connection")
	}
}

func TestSessionInputStops(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()

	input, err := newSessionInput(reader)
	if err != nil {
		t.Fatal(err)
	}

	go func() { _, _ = writer.Write([]byte("ls\n")) }()

	buf := make([]byte, 8)
	if n, err := input.Read(buf); err != nil || string(buf[:n]) != "ls\n" {
		t.Fatalf("read %q: %v", buf[:n], err)
	}

	input.stop()

	if n, err := input.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
		t.Errorf("a stopped input read %d bytes: %v", n, err)
	}
}

func TestLabRuntimeDescribe(t *testing.T) {
	rt := &fakeLabRuntime{labs: []*clablabruntime.LabState{fabricLabState("")}}
	b := newLabRuntimeBackend(t, rt)

	text, err := b.Inspect(t.Context(), clabinternaltuibackend.NodeRef{Lab: "fabric", Name: "R1"})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Workload", "namespace:", "c9s-fabric", "r1-7cfd5475fd-gq5b5", "node-r1", "uid-r1",
		"172.20.20.2/24",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the description lacks %q:\n%s", want, text)
		}
	}

	// The namespace is no network.
	if strings.Contains(text, "network:") {
		t.Errorf("the namespace is listed as a network:\n%s", text)
	}
}

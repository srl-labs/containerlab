// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fake

import (
	"errors"
	"testing"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

func newTestBackend() *Backend {
	clock := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)

	return New(Options{Now: func() time.Time { return clock }, Seed: 7})
}

func labNamed(t *testing.T, b *Backend, name string) clabinternaltuibackend.Lab {
	t.Helper()

	for _, lab := range b.Labs() {
		if lab.Name == name {
			return lab
		}
	}

	t.Fatalf("lab %q not found", name)

	return clabinternaltuibackend.Lab{}
}

// tickUntilDone advances the simulation until the job has finished.
func tickUntilDone(
	t *testing.T,
	b *Backend,
	id clabinternaltuibackend.JobID,
) clabinternaltuibackend.JobInfo {
	t.Helper()

	for range 100 {
		b.Tick()

		if job, ok := b.Job(id); ok && job.State.Done() {
			return job
		}
	}

	t.Fatalf("job %d did not finish", id)

	return clabinternaltuibackend.JobInfo{}
}

func TestDemoDataIsConsistent(t *testing.T) {
	b := newTestBackend()

	dc1 := labNamed(t, b, "dc1")
	if !dc1.Deployed || len(dc1.Nodes) != 10 || dc1.State() != clabinternaltuibackend.LabPartial {
		t.Fatalf(
			"unexpected dc1: deployed=%v nodes=%d state=%v",
			dc1.Deployed,
			len(dc1.Nodes),
			dc1.State(),
		)
	}

	if clos := labNamed(
		t,
		b,
		"clos",
	); clos.Deployed || len(clos.Nodes) != 0 ||
		clos.DefinedNodes != 6 {
		t.Errorf("unexpected clos: %+v", clos)
	}

	// Every interface of a link names a peer that exists and points back at it.
	for i := range dc1.Nodes {
		for _, iface := range b.Interfaces(dc1.Nodes[i].NodeRef) {
			if iface.Peer == nil {
				continue
			}

			peer := dc1.Node(iface.Peer.Node)
			if peer == nil {
				t.Errorf(
					"%s:%s points at unknown node %s",
					iface.Node.Name,
					iface.Name,
					iface.Peer.Node,
				)

				continue
			}

			found := false

			for _, other := range b.Interfaces(peer.NodeRef) {
				if other.Name == iface.Peer.Interface && other.Peer != nil &&
					other.Peer.Node == iface.Node.Name && other.Peer.Interface == iface.Name {
					found = true
				}
			}

			if !found {
				t.Errorf(
					"%s:%s has no matching far end on %s",
					iface.Node.Name,
					iface.Name,
					peer.Name,
				)
			}
		}
	}

	topo := b.Topology("dc1")
	if len(topo.Nodes) != 10 || len(topo.Links) != 12 {
		t.Errorf("the topology of dc1 has %d nodes and %d links", len(topo.Nodes), len(topo.Links))
	}

	if got := b.Topology("nope"); got.Warning == "" {
		t.Error("an unknown lab should come with a warning")
	}
}

func TestSimulationIsReproducibleAndKeepsHistoryBounded(t *testing.T) {
	a, b := newTestBackend(), newTestBackend()

	for range seriesLen * 2 {
		a.Tick()
		b.Tick()
	}

	na, nb := labNamed(t, a, "dc1").Nodes[0], labNamed(t, b, "dc1").Nodes[0]

	if na.CPUPercent != nb.CPUPercent || na.RxBps != nb.RxBps {
		t.Error("two backends with the same seed diverged")
	}

	if len(na.CPUSeries) != seriesLen || len(na.RxSeries) != seriesLen {
		t.Errorf(
			"the history holds %d and %d samples, want %d",
			len(na.CPUSeries),
			len(na.RxSeries),
			seriesLen,
		)
	}

	if !na.HasStats || na.CPUPercent <= 0 {
		t.Errorf("a running node has no load: %+v", na)
	}
}

func TestSnapshotsAreNotChangedByLaterTicks(t *testing.T) {
	b := newTestBackend()

	before := labNamed(t, b, "dc1")
	series := before.Nodes[0].CPUSeries
	first, last := series[0], series[len(series)-1]

	for range 5 {
		b.Tick()
	}

	if series[0] != first || series[len(series)-1] != last {
		t.Error("a history handed out earlier was modified by the simulation")
	}
}

func TestDeployAndDestroy(t *testing.T) {
	b := newTestBackend()

	id, err := b.Deploy(clabinternaltuibackend.DeployRequest{Lab: "clos"})
	if err != nil {
		t.Fatal(err)
	}

	if !labNamed(t, b, "clos").Busy {
		t.Error("a lab with a running job should be flagged busy")
	}

	// One operation per lab at a time.
	if _, err := b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "clos"}); !errors.Is(
		err, clabinternaltuibackend.ErrLabBusy) {
		t.Errorf("a second job for the lab should be refused, got %v", err)
	}

	job := tickUntilDone(t, b, id)
	if job.State != clabinternaltuibackend.JobSucceeded || job.Lines == 0 {
		t.Errorf("unexpected job: %+v", job)
	}

	if got := len(b.JobOutput(id, 0)); got != job.Lines {
		t.Errorf("the job reports %d lines, %d can be read", job.Lines, got)
	}

	if got := b.JobOutput(id, job.Lines); got != nil {
		t.Errorf("reading past the end returned %d lines", len(got))
	}

	clos := labNamed(t, b, "clos")
	if !clos.Deployed || len(clos.Nodes) != 6 || clos.Busy {
		t.Fatalf(
			"clos after the deploy: deployed=%v nodes=%d busy=%v",
			clos.Deployed,
			len(clos.Nodes),
			clos.Busy,
		)
	}

	if len(b.Interfaces(clos.Nodes[0].NodeRef)) == 0 {
		t.Error("the nodes of the deployed lab have no interfaces")
	}

	id, err = b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "clos"})
	if err != nil {
		t.Fatal(err)
	}

	tickUntilDone(t, b, id)

	if clos := labNamed(t, b, "clos"); clos.Deployed || len(clos.Nodes) != 0 {
		t.Errorf("clos after the destroy: %+v", clos)
	}
}

func TestStoppingANodeTakesItsLinksDown(t *testing.T) {
	b := newTestBackend()

	id, err := b.NodeAction(clabinternaltuibackend.NodeActionRequest{
		Lab: "dc1", Action: clabinternaltuibackend.NodeStop, Nodes: []string{"spine1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tickUntilDone(t, b, id)

	dc1 := labNamed(t, b, "dc1")
	if got := dc1.Node("spine1").Readiness(); got != clabinternaltuibackend.Stopped {
		t.Errorf("spine1 is %v, want it stopped", got)
	}

	if len(b.Interfaces(dc1.Node("spine1").NodeRef)) != 0 {
		t.Error("a stopped node should have no interfaces")
	}

	for _, iface := range b.Interfaces(dc1.Node("leaf2").NodeRef) {
		if iface.Peer != nil && iface.Peer.Node == "spine1" && iface.State != "down" {
			t.Errorf("leaf2:%s towards the stopped spine1 is %q", iface.Name, iface.State)
		}
	}

	// The nodes that kept running keep their history.
	if got := len(dc1.Node("leaf2").CPUSeries); got < seriesLen/2 {
		t.Errorf("leaf2 lost its history, %d samples left", got)
	}
}

func TestNetemAndCancel(t *testing.T) {
	b := newTestBackend()
	dc1 := labNamed(t, b, "dc1")
	leaf2 := dc1.Node("leaf2").NodeRef

	id, err := b.SetNetem(clabinternaltuibackend.NetemRequest{
		Interfaces: []clabinternaltuibackend.InterfaceRef{{Node: leaf2, Name: "e1-49"}},
		Impairment: clabinternaltuibackend.Impairment{Delay: 20 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}

	tickUntilDone(t, b, id)

	netem := func() *clabinternaltuibackend.Impairment {
		for _, iface := range b.Interfaces(leaf2) {
			if iface.Name == "e1-49" {
				return iface.Netem
			}
		}

		return nil
	}

	if got := netem(); got == nil || got.Delay != 20*time.Millisecond {
		t.Errorf("the impairment was not set: %+v", got)
	}

	id, _ = b.ResetNetem([]clabinternaltuibackend.InterfaceRef{{Node: leaf2, Name: "e1-49"}})
	tickUntilDone(t, b, id)

	if got := netem(); got != nil {
		t.Errorf("the impairment was not cleared: %+v", got)
	}

	// A cancelled job ends as cancelled and does not apply its effect.
	id, _ = b.Destroy(clabinternaltuibackend.DestroyRequest{Lab: "sp-core"})
	b.CancelJob(id)

	if job := tickUntilDone(t, b, id); job.State != clabinternaltuibackend.JobCancelled {
		t.Errorf("the job ended as %v", job.State)
	}

	if !labNamed(t, b, "sp-core").Deployed {
		t.Error("a cancelled destroy removed the lab")
	}

	// Stopping a capture is its regular end.
	id, err = b.Capture(clabinternaltuibackend.CaptureRequest{
		Lab:       "dc1",
		Wireshark: true,
		Targets: []clabinternaltuibackend.CaptureTarget{
			{Node: "leaf2", Interfaces: []string{"e1-49"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	b.Tick()

	if job, _ := b.Job(id); job.State.Done() || job.Bytes == 0 {
		t.Errorf("a capture should run and grow until stopped: %+v", job)
	}

	b.CancelJob(id)

	if job, _ := b.Job(id); job.State != clabinternaltuibackend.JobSucceeded {
		t.Errorf("a stopped capture ended as %v", job.State)
	}
}

func TestChangesAreSignaled(t *testing.T) {
	b := newTestBackend()
	b.Changed()

	select {
	case <-b.Notify():
	default:
	}

	b.Tick()

	select {
	case <-b.Notify():
	default:
		t.Fatal("a step of the simulation did not wake the UI")
	}

	if topics := b.Changed(); !topics.Has(clabinternaltuibackend.TopicStats) {
		t.Errorf("the changed topics are %b, want the stats among them", topics)
	}

	if topics := b.Changed(); topics != 0 {
		t.Errorf("reading the changes should reset them, %b left", topics)
	}
}

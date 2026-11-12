// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	leaf1Container = "clab-lab1-leaf1"
	leaf2Container = "clab-lab1-leaf2"
)

// replayEvents applies the recorded event stream and returns which events changed the
// known state.
func replayEvents(t *testing.T, states *ifaceStates) []bool {
	t.Helper()

	f, err := os.Open("testdata/events.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var changes []bool

	err = decodeEvents(f, func(ev *streamEvent) {
		changes = append(changes, states.apply(ev))
	})
	if err != nil {
		t.Fatal(err)
	}

	return changes
}

func TestDecodeEvents(t *testing.T) {
	var events []streamEvent

	f, err := os.Open("testdata/events.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := decodeEvents(f, func(ev *streamEvent) { events = append(events, *ev) }); err != nil {
		t.Fatal(err)
	}

	if len(events) != 13 {
		t.Fatalf("decoded %d events, want 13", len(events))
	}

	netem := events[4]
	if netem.Type != eventTypeInterface || netem.Action != "snapshot" ||
		netem.container() != leaf1Container || netem.Attributes["netem_loss"] != "1.50%" {
		t.Errorf("unexpected event: %+v", netem)
	}

	want := time.Date(2026, 3, 2, 9, 0, 0, 10200000, time.UTC)
	if !netem.Timestamp.Equal(want) {
		t.Errorf("timestamp = %v, want %v", netem.Timestamp, want)
	}

	// An event without an actor name still names its container in the attributes.
	ev := streamEvent{Attributes: map[string]string{"name": "clab-x-y"}}
	if ev.container() != "clab-x-y" {
		t.Errorf("container() = %q", ev.container())
	}
}

func TestDecodeEventsStopsAtGarbage(t *testing.T) {
	input := `{"type":"interface","action":"create","actor_name":"c","attributes":{"ifname":"e1"}}
this is not JSON
{"type":"interface","action":"create","actor_name":"c","attributes":{"ifname":"e2"}}
`
	seen := 0

	err := decodeEvents(strings.NewReader(input), func(*streamEvent) { seen++ })
	if err == nil || seen != 1 {
		t.Errorf("err = %v after %d events, want an error after 1", err, seen)
	}
}

func TestInterfaceStateFromEvents(t *testing.T) {
	var states ifaceStates

	wantChanges := []bool{
		false, false, // the containers are listed
		true, true, true, true, true, // the interfaces are listed
		false,      // a command runs in a container
		true, true, // an interface changes, another one appears
		false, // statistics are not state
		true,  // an interface goes away
		true,  // a container stops
	}

	if diff := cmp.Diff(wantChanges, replayEvents(t, &states)); diff != "" {
		t.Errorf("unexpected changes (-want +got):\n%s", diff)
	}

	if got := states.of(leaf2Container); len(got) != 0 {
		t.Errorf("a stopped container kept its interfaces: %+v", got)
	}

	got := map[string]ifaceState{}
	for _, st := range states.of(leaf1Container) {
		got[st.name] = st
	}

	want := map[string]ifaceState{
		"lo": {name: "lo", mac: "00:00:00:00:00:00", kind: "device", mtu: 65536, index: 1},
		"e1-1": {
			name: "e1-1", alias: "ethernet-1/1", mac: "aa:c1:ab:76:42:b0", kind: "veth",
			mtu: 9232, index: 22, state: stateDown,
			netem: &clabinternaltuibackend.Impairment{Delay: 250 * time.Millisecond},
		},
		"e1-2": {
			name: "e1-2", mac: "aa:c1:ab:11:22:33", kind: "veth", mtu: 9232, index: 26,
			state: stateUp,
		},
		"clab-1a2b3c4d": {
			name: "clab-1a2b3c4d", alias: "eth3", mac: "aa:c1:ab:00:00:57", kind: "veth",
			mtu: 9500, index: 57, state: stateUp,
		},
	}

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(ifaceState{})); diff != "" {
		t.Errorf("unexpected interface states (-want +got):\n%s", diff)
	}

	// Applying the same state again changes nothing.
	again := streamEvent{
		Type: eventTypeInterface, Action: "update", ActorName: leaf1Container,
		Attributes: map[string]string{
			"ifname": "e1-2", "index": "26", "mtu": "9232", "state": "up", "type": "veth",
			"mac": "aa:c1:ab:11:22:33",
		},
	}
	if states.apply(&again) {
		t.Error("an event that repeats the known state was reported as a change")
	}

	if !states.retain(map[string]struct{}{}) || len(states.of(leaf1Container)) != 0 {
		t.Error("the state of a container that is gone was kept")
	}
}

func TestInterfaceImpairmentsFromEvents(t *testing.T) {
	st := ifaceStateFromAttributes(map[string]string{
		"ifname": "e1-1", "state": "up",
		"netem_delay": "100ms", "netem_jitter": "10ms", "netem_loss": "1.50%",
		"netem_rate": "1000kbit", "netem_corruption": "0.10%",
	})

	want := &clabinternaltuibackend.Impairment{
		Delay: 100 * time.Millisecond, Jitter: 10 * time.Millisecond,
		LossPct: 1.5, RateKbit: 1000, CorruptionPct: 0.1,
	}

	if diff := cmp.Diff(want, st.netem); diff != "" {
		t.Errorf("unexpected impairments (-want +got):\n%s", diff)
	}

	for state, want := range map[string]string{
		"up": stateUp, "UP": stateUp, "down": stateDown, "lowerlayerdown": stateDown,
		"dormant": stateDown, "unknown": "", "": "",
	} {
		if got := operState(state); got != want {
			t.Errorf("operState(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestMergeInterfaces(t *testing.T) {
	var states ifaceStates

	replayEvents(t, &states)

	node := clabinternaltuibackend.NodeRef{Lab: "lab1", Name: "leaf1", Container: leaf1Container}
	sampled := map[string]clabinternaltuibackend.Interface{
		"e1-1":  {Name: "e1-1", HasStats: true, RxBps: 4000, RxBytes: 1000},
		"e1-2":  {Name: "e1-2", HasStats: true},
		"eth0":  {Name: "eth0", HasStats: true, TxBytes: 7},
		"eth3":  {Name: "eth3", HasStats: true, RxBytes: 3},
		"mgmt0": {Name: "mgmt0", HasStats: true},
	}

	got := mergeInterfaces(node, sampled, states.of(leaf1Container))
	want := []clabinternaltuibackend.Interface{
		{
			Node: node, Name: "e1-1", Alias: "ethernet-1/1", MAC: "aa:c1:ab:76:42:b0",
			Type: "veth", MTU: 9232, Index: 22, State: stateDown,
			Netem:    &clabinternaltuibackend.Impairment{Delay: 250 * time.Millisecond},
			HasStats: true, RxBps: 4000, RxBytes: 1000,
		},
		{
			Node: node, Name: "e1-2", MAC: "aa:c1:ab:11:22:33", Type: "veth", MTU: 9232,
			Index: 26, State: stateUp, HasStats: true,
		},
		// The event stream no longer knows the interface, its counters are still read.
		{Node: node, Name: "eth0", HasStats: true, TxBytes: 7},
		// A link keeps this interface on the host and reports it under the name it has
		// there. It is listed under the name the node knows.
		{
			Node: node, Name: "eth3", MAC: "aa:c1:ab:00:00:57", Type: "veth", MTU: 9500,
			Index: 57, State: stateUp, HasStats: true, RxBytes: 3,
		},
		{Node: node, Name: "mgmt0", HasStats: true},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected interfaces (-want +got):\n%s", diff)
	}

	// Without samples the state alone is listed, and without state the samples alone.
	stateOnly := mergeInterfaces(node, nil, states.of(leaf1Container))

	names := make([]string, 0, len(stateOnly))

	for _, iface := range stateOnly {
		names = append(names, iface.Name)

		if iface.HasStats {
			t.Errorf("%s has statistics without a sample", iface.Name)
		}
	}

	if diff := cmp.Diff([]string{"clab-1a2b3c4d", "e1-1", "e1-2"}, names); diff != "" {
		t.Errorf("unexpected interfaces from state alone (-want +got):\n%s", diff)
	}

	if got := mergeInterfaces(node, sampled, nil); len(got) != len(sampled) || got[0].State != "" {
		t.Errorf("unexpected interfaces from samples alone: %+v", got)
	}
}

func TestInterfaceStateStream(t *testing.T) {
	b := newTestBackend(t)

	if caps := b.Host().Caps; caps.LinkState || caps.Reason(capLinkState) == "" {
		t.Errorf("link state before the stream runs: %+v", caps)
	}

	result := make(chan error, 1)

	go func() { result <- b.runIfaceStream(context.Background()) }()

	proc := b.run.proc(t, 0)

	want := argv("events -r docker --format json --initial-state")
	if diff := cmp.Diff(want, proc.spec.args); diff != "" {
		t.Errorf("unexpected arguments (-want +got):\n%s", diff)
	}

	eventually(t, "the link state capability", func() bool { return b.Host().Caps.LinkState })
	b.drainTopics()

	events, err := os.ReadFile("testdata/events.ndjson")
	if err != nil {
		t.Fatal(err)
	}

	// The stream arrives in pieces that do not end at line breaks.
	half := len(events) / 2
	proc.stdout(string(events[:half]))
	proc.stdout(string(events[half:]))

	eventually(t, "the events to be applied", func() bool {
		return len(b.ifaces.of(leaf1Container)) == 4 && len(b.ifaces.of(leaf2Container)) == 0
	})

	if topics := b.drainTopics(); !topics.Has(clabinternaltuibackend.TopicInterfaces) {
		t.Errorf("interface events signaled %b", topics)
	}

	proc.stderr("10:00:00 INFO not worth a message\n")
	proc.stderr("10:00:01 WARN netlink subscription ended, resubscribing\n")
	proc.stderr("   ERROR  \n  permission denied.\n")
	proc.exitWith(1)

	if err := <-result; err == nil || err.Error() != "permission denied." {
		t.Errorf("err = %v, want the error the command printed", err)
	}

	logged := b.Messages()
	messages := make([]string, 0, len(logged))

	for _, msg := range logged {
		messages = append(messages, msg.Level+" "+msg.Text)
	}

	wantMessages := []string{"WARN events: netlink subscription ended, resubscribing"}
	if diff := cmp.Diff(wantMessages, messages); diff != "" {
		t.Errorf("unexpected messages (-want +got):\n%s", diff)
	}
}

func TestInterfaceStateStreamStopsWithTheBackend(t *testing.T) {
	b := newTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() { result <- b.runIfaceStream(ctx) }()

	proc := b.run.proc(t, 0)
	proc.onSignal = func(syscall.Signal) { proc.exitWith(0) }

	cancel()

	select {
	case <-result:
	case <-time.After(testTimeout):
		t.Fatal("the stream did not stop")
	}

	if diff := cmp.Diff([]syscall.Signal{syscall.SIGINT}, proc.received()); diff != "" {
		t.Errorf("unexpected signals (-want +got):\n%s", diff)
	}
}

func TestInterfaceStateStreamRestartsOnGarbage(t *testing.T) {
	b := newTestBackend(t)
	result := make(chan error, 1)

	go func() { result <- b.runIfaceStream(context.Background()) }()

	proc := b.run.proc(t, 0)
	proc.onSignal = func(syscall.Signal) { proc.exitWith(-1) }
	proc.stdout("Usage: containerlab events [flags]\n")

	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "unreadable event") {
			t.Errorf("err = %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("the stream was not ended")
	}

	if diff := cmp.Diff([]syscall.Signal{syscall.SIGKILL}, proc.received()); diff != "" {
		t.Errorf("unexpected signals (-want +got):\n%s", diff)
	}
}

func TestInterfaceStateStreamIsRestarted(t *testing.T) {
	b := newTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		b.ifaceStream(ctx)
	}()

	first := b.run.proc(t, 0)
	first.stdout(`{"type":"interface","action":"create","actor_name":"c1",` +
		`"attributes":{"ifname":"eth1","state":"up"}}` + "\n")

	eventually(t, "the event to be applied", func() bool { return len(b.ifaces.of("c1")) == 1 })

	first.stderr("10:00:00 ERRO lost the runtime\n")
	first.exitWith(1)

	// What was known is dropped with the stream, and the state is unknown until the next
	// stream runs.
	eventually(t, "the state to be dropped", func() bool {
		caps := b.Host().Caps

		return len(b.ifaces.of("c1")) == 0 && !caps.LinkState &&
			strings.Contains(caps.Reason(capLinkState), "restarting in 1s: lost the runtime")
	})

	second := b.run.proc(t, 1)
	second.onSignal = func(syscall.Signal) { second.exitWith(0) }

	eventually(t, "the link state capability", func() bool { return b.Host().Caps.LinkState })

	cancel()

	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("the stream loop did not stop")
	}

	if b.run.count() != 2 {
		t.Errorf("%d streams were started, want 2", b.run.count())
	}
}

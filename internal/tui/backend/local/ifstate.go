// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuitermtext "github.com/srl-labs/containerlab/internal/tui/termtext"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	eventTypeInterface = "interface"

	// Actions of the interface events.
	ifaceActionDelete = "delete"
	ifaceActionStats  = "stats"

	stateUp   = "up"
	stateDown = "down"

	ifaceStreamBackoffFirst = time.Second
	ifaceStreamBackoffLimit = 30 * time.Second
	// ifaceStreamStop is how long the stream command gets to exit after an interrupt.
	ifaceStreamStop = 2 * time.Second
)

// streamEvent is an event as the events command prints it in its JSON format.
type streamEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ActorName  string            `json:"actor_name"`
	Attributes map[string]string `json:"attributes"`
}

// container returns the name of the container the event is about.
func (e *streamEvent) container() string {
	if e.ActorName != "" {
		return e.ActorName
	}

	return e.Attributes["name"]
}

// decodeEvents reads a stream of JSON events until it ends and hands each event to handle.
func decodeEvents(r io.Reader, handle func(ev *streamEvent)) error {
	dec := json.NewDecoder(r)

	for {
		var ev streamEvent

		err := dec.Decode(&ev)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		handle(&ev)
	}
}

// ifaceState is what the event stream told about an interface.
type ifaceState struct {
	name  string
	alias string
	mac   string
	kind  string
	mtu   int
	index int
	// state is "up", "down" or empty when unknown.
	state string
	netem *clabinternaltuibackend.Impairment
}

// ifaceStateFromAttributes maps the attributes of an interface event.
func ifaceStateFromAttributes(attrs map[string]string) ifaceState {
	st := ifaceState{
		name:  attrs["ifname"],
		alias: attrs["alias"],
		mac:   attrs["mac"],
		kind:  attrs["type"],
		state: operState(attrs["state"]),
	}

	st.mtu, _ = strconv.Atoi(attrs["mtu"])
	st.index, _ = strconv.Atoi(attrs["index"])

	var netem clabinternaltuibackend.Impairment

	netem.Delay, _ = time.ParseDuration(attrs["netem_delay"])
	netem.Jitter, _ = time.ParseDuration(attrs["netem_jitter"])
	netem.LossPct, _ = strconv.ParseFloat(strings.TrimSuffix(attrs["netem_loss"], "%"), 64)
	netem.RateKbit, _ = strconv.ParseUint(strings.TrimSuffix(attrs["netem_rate"], "kbit"), 10, 64)
	netem.CorruptionPct, _ = strconv.ParseFloat(
		strings.TrimSuffix(attrs["netem_corruption"], "%"), 64)

	if !netem.IsZero() {
		st.netem = &netem
	}

	return st
}

// operState reduces the operational state of an interface to up, down or unknown. Virtual
// interfaces commonly do not report a state at all.
func operState(state string) string {
	switch strings.ToLower(state) {
	case stateUp:
		return stateUp
	case "", "unknown":
		return ""
	default:
		return stateDown
	}
}

// ifaceStates holds the interface state learned from the event stream, by container.
type ifaceStates struct {
	mu         sync.RWMutex
	containers map[string]map[string]ifaceState
}

// apply updates the state with an event and reports whether it changed something.
func (s *ifaceStates) apply(ev *streamEvent) bool {
	container := ev.container()
	if container == "" {
		return false
	}

	if ev.Type != eventTypeInterface {
		s.mu.Lock()
		defer s.mu.Unlock()

		return s.applyContainerEvent(container, ev.Action)
	}

	st := ifaceStateFromAttributes(ev.Attributes)

	return s.applyInterface(container, ev.Action, &st)
}

// applyInterface updates the state of an interface of a container with what an event of
// the given action told about it, and reports whether that changed something.
func (s *ifaceStates) applyInterface(container, action string, st *ifaceState) bool {
	if st.name == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ifaces := s.containers[container]

	switch action {
	case ifaceActionStats:
		// The counters are kept elsewhere, the state did not change.
		return false
	case ifaceActionDelete:
		if _, ok := ifaces[st.name]; !ok {
			return false
		}

		delete(ifaces, st.name)

		return true
	}

	if previous, ok := ifaces[st.name]; ok && sameIfaceState(&previous, st) {
		return false
	}

	if s.containers == nil {
		s.containers = map[string]map[string]ifaceState{}
	}

	if ifaces == nil {
		ifaces = map[string]ifaceState{}
		s.containers[container] = ifaces
	}

	ifaces[st.name] = *st

	return true
}

// applyContainerEvent drops the interfaces of a container that stopped: they went away
// with its network namespace. The lock must be held.
func (s *ifaceStates) applyContainerEvent(container, action string) bool {
	switch action {
	case clabruntime.EventActionDie, clabruntime.EventActionStop,
		clabruntime.EventActionDestroy, clabruntime.EventActionKill, "exited":
	default:
		return false
	}

	if _, ok := s.containers[container]; !ok {
		return false
	}

	delete(s.containers, container)

	return true
}

func sameIfaceState(a, b *ifaceState) bool {
	if (a.netem == nil) != (b.netem == nil) || (a.netem != nil && *a.netem != *b.netem) {
		return false
	}

	return a.name == b.name && a.alias == b.alias && a.mac == b.mac && a.kind == b.kind &&
		a.mtu == b.mtu && a.index == b.index && a.state == b.state
}

// retain forgets the containers that are not in the given set of running ones and reports
// whether there was something to forget.
func (s *ifaceStates) retain(running map[string]struct{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	dropped := false

	for container := range s.containers {
		if _, ok := running[container]; !ok {
			delete(s.containers, container)

			dropped = true
		}
	}

	return dropped
}

// reset forgets everything and reports whether there was something to forget.
func (s *ifaceStates) reset() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	known := len(s.containers) > 0
	s.containers = nil

	return known
}

// of returns the known interface states of a container.
func (s *ifaceStates) of(container string) []ifaceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]ifaceState, 0, len(s.containers[container]))
	for _, st := range s.containers[container] {
		states = append(states, st)
	}

	return states
}

// mergeInterfaces joins the interfaces sampled from /proc with the state the event stream
// reported for the same container. An interface is matched by its name; one that a link
// keeps on the host side of the node is reported under that host name with the name the
// node knows as alias, and is matched by the alias. Interfaces known to one source only are
// kept, without the values the other source would add.
func mergeInterfaces(
	node clabinternaltuibackend.NodeRef,
	sampled map[string]clabinternaltuibackend.Interface,
	states []ifaceState,
) []clabinternaltuibackend.Interface {
	out := make([]clabinternaltuibackend.Interface, 0, len(sampled)+len(states))
	used := map[string]bool{}

	// A fixed order makes the outcome stable should two states claim the same interface.
	slices.SortFunc(states, func(a, b ifaceState) int {
		return strings.Compare(a.name, b.name)
	})

	for i := range states {
		st := &states[i]

		name := st.name
		if _, ok := sampled[name]; !ok {
			if _, byAlias := sampled[st.alias]; byAlias {
				name = st.alias
			}
		}

		if name == loopbackInterface || used[name] {
			continue
		}

		// A missing sample leaves the zero value, an interface without statistics.
		iface := sampled[name]
		used[name] = true

		iface.Name = name
		iface.MAC = st.mac
		iface.Type = st.kind
		iface.MTU = st.mtu
		iface.Index = st.index
		iface.State = st.state

		if st.alias != name {
			iface.Alias = st.alias
		}

		if st.netem != nil {
			netem := *st.netem
			iface.Netem = &netem
		}

		out = append(out, iface)
	}

	for name := range sampled {
		if !used[name] {
			out = append(out, sampled[name])
		}
	}

	for i := range out {
		out[i].Node = node
	}

	slices.SortFunc(out, func(a, b clabinternaltuibackend.Interface) int {
		return clabinternaltuitermtext.NaturalCompare(a.Name, b.Name)
	})

	return out
}

// Interfaces returns the interfaces of a node: the counters and rates sampled from /proc,
// the state and impairments reported by the event stream, and the far end of the link each
// interface belongs to. For a node that a lab runtime runs, the counters come with the
// events of that runtime as well.
func (b *Backend) Interfaces(
	ref clabinternaltuibackend.NodeRef,
) []clabinternaltuibackend.Interface {
	rec, node, _ := b.inv.find(ref)
	if node == nil {
		return nil
	}

	key := rec.stateKey(node.Container)

	ifaces := mergeInterfaces(node.NodeRef, b.stats.interfaces(key), b.ifaces.of(key))
	if len(ifaces) == 0 {
		return ifaces
	}

	topo := b.deployedTopology(rec)

	for i := range ifaces {
		ifaces[i].Peer = peerOf(&topo, node.Name, &ifaces[i])
	}

	return ifaces
}

// peerOf finds the far end of the link an interface of a node belongs to. Topologies name
// an interface either the way the node's operating system does or by its Linux name, so
// both the name and the alias are tried.
func peerOf(
	topo *clabinternaltuibackend.Topology,
	node string,
	iface *clabinternaltuibackend.Interface,
) *clabinternaltuibackend.Endpoint {
	matches := func(ep *clabinternaltuibackend.Endpoint) bool {
		return ep.Node == node && ep.Interface != "" &&
			(ep.Interface == iface.Name || ep.Interface == iface.Alias)
	}

	for i := range topo.Links {
		link := &topo.Links[i]
		if link.Z == nil {
			continue
		}

		var peer clabinternaltuibackend.Endpoint

		switch {
		case matches(&link.A):
			peer = *link.Z
		case matches(link.Z):
			peer = link.A
		default:
			continue
		}

		return &peer
	}

	return nil
}

// ifaceStream runs the events command, which reports the state of the interfaces of every
// lab, until ctx ends. The command reads the network namespaces of the nodes and needs
// root privileges for that, which is why it is a child process. It is restarted when it
// exits, after a growing pause.
func (b *Backend) ifaceStream(ctx context.Context) {
	pause := newBackoff(ifaceStreamBackoffFirst, ifaceStreamBackoffLimit)

	for ctx.Err() == nil {
		started := time.Now()
		err := b.runIfaceStream(ctx)

		// Without the stream the states go stale, and a new stream reports them all again.
		if b.ifaces.reset() {
			b.mark(clabinternaltuibackend.TopicInterfaces)
		}

		if ctx.Err() != nil {
			return
		}

		if time.Since(started) > streamHealthy {
			pause.reset()
		}

		wait := pause.next()
		reason := fmt.Sprintf("the interface state stream stopped, restarting in %s: %v",
			wait, err)

		b.setLinkState(false, reason)
		b.logf(levelWarn, "%s", reason)

		if !sleep(ctx, wait) {
			return
		}
	}
}

// runIfaceStream runs the events command once and applies what it prints. It returns when
// the command exited, with the reason as far as one is known.
func (b *Backend) runIfaceStream(ctx context.Context) error {
	reader, writer := io.Pipe()

	var lastErr stderrTail

	errWriter := &lineWriter{emit: func(line string) { b.ifaceStreamMessage(&lastErr, line) }}

	proc, err := b.run.start(&procSpec{
		path:   b.exe,
		args:   eventsArgs(b.rtName),
		env:    childEnv(b.environ()),
		stdout: writer,
		stderr: errWriter,
	})
	if err != nil {
		return err
	}

	b.setLinkState(true, "")

	decoded := make(chan error, 1)

	b.spawn("interface state decoder", func() {
		err := decodeEvents(reader, b.applyIfaceEvent)
		decoded <- err

		if err != nil {
			// The stream cannot be followed any longer. The command holds no state worth
			// a clean exit, so it is killed and started anew.
			_ = proc.signal(syscall.SIGKILL)
		}

		// Keep reading until the command is gone, it must not block on its output.
		_, _ = io.Copy(io.Discard, reader)
	})

	exited := make(chan struct{})

	b.spawn("interface state stream stop", func() {
		select {
		case <-ctx.Done():
			stopProcess(proc, exited, ifaceStreamStop)
		case <-exited:
		}
	})

	code, err := proc.wait()

	close(exited)
	writer.Close()
	errWriter.close()

	if decodeErr := <-decoded; decodeErr != nil {
		return fmt.Errorf("unreadable event: %w", decodeErr)
	}

	switch {
	case err != nil:
		return err
	case lastErr.text != "":
		return errors.New(lastErr.text)
	default:
		return fmt.Errorf("exit status %d", code)
	}
}

// stopProcess interrupts a process and kills it if it has not exited within the grace
// period. exited is closed once the process is gone.
func stopProcess(proc process, exited <-chan struct{}, grace time.Duration) {
	_ = proc.signal(syscall.SIGINT)

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case <-exited:
	case <-timer.C:
		_ = proc.signal(syscall.SIGKILL)
	}
}

func (b *Backend) applyIfaceEvent(ev *streamEvent) {
	if b.ifaces.apply(ev) {
		b.mark(clabinternaltuibackend.TopicInterfaces)
	}
}

// stderrTail remembers the last problem a command reported.
type stderrTail struct {
	text string
}

// ifaceStreamMessage passes the warnings and errors of the events command on to the
// messages of the backend.
func (b *Backend) ifaceStreamMessage(last *stderrTail, raw string) {
	line := clabinternaltuitermtext.Sanitize(raw)

	_, level, msg, ok := parseLogLine(line)
	if !ok {
		// What is not a log record is the error the command ended with.
		if text := strings.TrimSpace(line); text != "" && text != errorBanner {
			last.text = text
		}

		return
	}

	if level != levelWarn && !isErrorLevel(level) {
		return
	}

	if isErrorLevel(level) {
		last.text = msg
	}

	b.logf(level, "events: %s", msg)
}

// setLinkState records whether the state of the interfaces is known.
func (b *Backend) setLinkState(known bool, reason string) {
	b.mu.Lock()

	changed := b.host.Caps.LinkState != known || b.host.Caps.Reasons[capLinkState] != reason
	b.host.Caps.LinkState = known

	if known {
		delete(b.host.Caps.Reasons, capLinkState)
	} else {
		b.host.Caps.Reasons[capLinkState] = reason
	}

	b.mu.Unlock()

	if changed {
		b.mark(clabinternaltuibackend.TopicHost)
	}
}

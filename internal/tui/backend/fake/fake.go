// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package fake provides a backend that serves made-up labs with simulated traffic. It
// backs the demo mode of the TUI and its tests, neither of which needs a container
// runtime or root privileges.
package fake

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// seriesLen is the number of samples kept per value.
	seriesLen = 60
	// tickInterval is the pace of the simulation when it runs live.
	tickInterval = time.Second
	// bytesPerPacket is the packet size used to derive packet rates from bit rates.
	bytesPerPacket = 800
	bitsPerByte    = 8

	maxMessages = 200

	// pcgStream sets the second half of the seed of the random numbers apart from the first.
	pcgStream = 0x9e3779b97f4a7c15
	// returnShare is the traffic a link carries back, as a share of what it carries forth.
	returnShare = 0.93
)

// Options configures the fake backend.
type Options struct {
	// Live advances the simulation once a second after Start. Tests leave it off and
	// call Tick themselves.
	Live bool
	// Now is the clock, time.Now when nil.
	Now func() time.Time
	// Seed makes the simulated values reproducible.
	Seed uint64
	// FabricLeaves deploys the lab named "fabric" with that many leaves, four spines and
	// four hosts behind every leaf, to try the UI at scale. When zero the fabric is
	// listed with its default size, but not deployed.
	FabricLeaves int
}

// Backend is an in-memory clabinternaltuibackend.Backend.
type Backend struct {
	mu   sync.Mutex
	opts Options
	rng  *rand.Rand

	specs  []labSpec
	labs   []clabinternaltuibackend.Lab
	ifaces map[string][]clabinternaltuibackend.Interface
	links  []*simLink

	jobs    []*job
	nextJob clabinternaltuibackend.JobID

	messages []clabinternaltuibackend.LogLine
	// sessions lists the interactive sessions that were asked for.
	sessions []string

	notify  chan struct{}
	changed clabinternaltuibackend.Topic
	stop    context.CancelFunc
	wg      sync.WaitGroup
}

var _ clabinternaltuibackend.Backend = (*Backend)(nil)

// simLink is the traffic state of a link.
type simLink struct {
	lab  string
	spec *linkSpec
	// ab and ba are the current rates in each direction, in bits per second.
	ab, ba float64
}

// New returns a backend preloaded with the demo labs.
func New(opts Options) *Backend {
	if opts.Now == nil {
		opts.Now = time.Now
	}

	b := &Backend{
		opts:   opts,
		rng:    rand.New(rand.NewPCG(opts.Seed, opts.Seed^pcgStream)), //nolint:gosec
		specs:  demoSpecs(opts.FabricLeaves),
		ifaces: map[string][]clabinternaltuibackend.Interface{},
		notify: make(chan struct{}, 1),
	}

	for i := range b.specs {
		b.labs = append(b.labs, b.buildLab(i))
	}

	// Fill the history, so that the charts have something to show from the start.
	for range seriesLen {
		b.simulate()
	}

	b.logf("INFO", "demo mode: the labs and their traffic are simulated")

	return b
}

func (b *Backend) buildLab(index int) clabinternaltuibackend.Lab {
	spec := &b.specs[index]
	lab := clabinternaltuibackend.Lab{
		Name:         spec.name,
		TopoFile:     spec.topoFile,
		LabDir:       filepath.Join(filepath.Dir(spec.topoFile), "clab-"+spec.name),
		Deployed:     spec.deployed,
		DefinedNodes: len(spec.nodes),
	}

	if !spec.deployed {
		return lab
	}

	lab.Owner = demoOwner
	lab.Shares = slices.Clone(spec.shares)

	for i := range spec.nodes {
		lab.Nodes = append(lab.Nodes, buildNode(spec, index, i, &spec.nodes[i]))
	}

	b.buildInterfaces(index, &lab)

	return lab
}

// buildInterfaces creates the interfaces of the running nodes of a lab and registers its
// links with the traffic simulation.
func (b *Backend) buildInterfaces(index int, lab *clabinternaltuibackend.Lab) {
	spec := &b.specs[index]

	b.links = slices.DeleteFunc(b.links, func(l *simLink) bool { return l.lab == spec.name })

	for i := range lab.Nodes {
		n := &lab.Nodes[i]
		delete(b.ifaces, n.Container)

		if n.State != stateRunning {
			continue
		}

		b.ifaces[n.Container] = []clabinternaltuibackend.Interface{{
			Node:  n.NodeRef,
			Name:  mgmtInterface(n.Kind),
			Type:  "veth",
			MTU:   mtuMgmt,
			Index: 1,
			State: "up",
			MAC:   fmt.Sprintf("02:42:ac:14:%02x:%02x", 20+index, 11+i), //nolint:mnd // made up
		}}
	}

	for i := range spec.links {
		l := &spec.links[i]
		b.links = append(
			b.links,
			&simLink{lab: spec.name, spec: l, ab: l.rate, ba: l.rate * returnShare},
		)

		b.addLinkEnd(lab, spec, index, i, l.a, l.aif, l.z, l.zif, false)
		b.addLinkEnd(lab, spec, index, i, l.z, l.zif, l.a, l.aif, true)
	}
}

func (b *Backend) addLinkEnd(
	lab *clabinternaltuibackend.Lab, spec *labSpec,
	labIndex, link int, node, iface, peerNode, peerIface string, zSide bool,
) {
	n := lab.Node(node)
	if n == nil || n.State != stateRunning {
		return
	}

	l := &spec.links[link]
	peer := lab.Node(peerNode)

	state := "up"
	if l.down || peer == nil || peer.State != stateRunning {
		state = "down"
	}

	entry := clabinternaltuibackend.Interface{
		Node:  n.NodeRef,
		Name:  iface,
		Alias: ifaceAlias(kindOf(spec, node), iface),
		Type:  "veth",
		MTU:   mtuFabric,
		Index: len(b.ifaces[n.Container]) + 1,
		State: state,
		MAC:   demoMAC(labIndex, link, zSide),
		Peer:  &clabinternaltuibackend.Endpoint{Node: peerNode, Interface: peerIface},
	}

	if !zSide && l.netem != nil {
		netem := *l.netem
		entry.Netem = &netem
	}

	b.ifaces[n.Container] = append(b.ifaces[n.Container], entry)
}

// Start implements clabinternaltuibackend.Backend.
func (b *Backend) Start(ctx context.Context) error {
	if !b.opts.Live {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)

	b.mu.Lock()
	b.stop = cancel
	b.mu.Unlock()

	b.wg.Go(func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.Tick()
			}
		}
	})

	return nil
}

// Close implements clabinternaltuibackend.Backend.
func (b *Backend) Close() {
	b.mu.Lock()
	stop := b.stop
	b.stop = nil
	b.mu.Unlock()

	if stop != nil {
		stop()
	}

	b.wg.Wait()
}

// Tick advances the simulation by one step: traffic and load move on, and running jobs
// make progress.
func (b *Backend) Tick() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.simulate()
	b.advanceJobs()
	b.signal(clabinternaltuibackend.TopicStats)
}

// signal records changed topics and wakes the UI. The lock must be held.
func (b *Backend) signal(topics clabinternaltuibackend.Topic) {
	b.changed |= topics

	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// Notify implements clabinternaltuibackend.Backend.
func (b *Backend) Notify() <-chan struct{} { return b.notify }

// Changed implements clabinternaltuibackend.Backend.
func (b *Backend) Changed() clabinternaltuibackend.Topic {
	b.mu.Lock()
	defer b.mu.Unlock()

	changed := b.changed
	b.changed = 0

	return changed
}

// SetFocus implements clabinternaltuibackend.Backend.
func (*Backend) SetFocus(clabinternaltuibackend.Focus) {}

// Refresh implements clabinternaltuibackend.Backend.
func (b *Backend) Refresh() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.signal(clabinternaltuibackend.TopicLabs)
}

// Host implements clabinternaltuibackend.Backend.
func (*Backend) Host() clabinternaltuibackend.Host {
	return clabinternaltuibackend.Host{
		Runtime:    "docker",
		Version:    "demo",
		Hostname:   "demo",
		User:       demoOwner,
		Privileged: true,
		Demo:       true,
		Caps: clabinternaltuibackend.Capabilities{
			Lifecycle: true,
			Netem:     true,
			Capture:   true,
			Share:     true,
			LinkState: true,
			Wireshark: true,
		},
	}
}

// Labs implements clabinternaltuibackend.Backend.
func (b *Backend) Labs() []clabinternaltuibackend.Lab {
	b.mu.Lock()
	defer b.mu.Unlock()

	labs := slices.Clone(b.labs)
	for i := range labs {
		labs[i].Nodes = slices.Clone(labs[i].Nodes)
		labs[i].Shares = slices.Clone(labs[i].Shares)
		labs[i].Busy = b.labBusy(labs[i].Name)
	}

	return labs
}

// Interfaces implements clabinternaltuibackend.Backend.
func (b *Backend) Interfaces(
	node clabinternaltuibackend.NodeRef,
) []clabinternaltuibackend.Interface {
	b.mu.Lock()
	defer b.mu.Unlock()

	return slices.Clone(b.ifaces[node.Container])
}

// Topology implements clabinternaltuibackend.Backend.
func (b *Backend) Topology(lab string) clabinternaltuibackend.Topology {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range b.specs {
		if b.specs[i].name == lab {
			return buildTopology(&b.specs[i])
		}
	}

	return clabinternaltuibackend.Topology{Lab: lab, Warning: "unknown lab"}
}

// Messages implements clabinternaltuibackend.Backend.
func (b *Backend) Messages() []clabinternaltuibackend.LogLine {
	b.mu.Lock()
	defer b.mu.Unlock()

	return slices.Clone(b.messages)
}

// logf records a backend message. The lock must be held, or the backend not shared yet.
func (b *Backend) logf(level, format string, args ...any) {
	b.messages = append(b.messages, clabinternaltuibackend.LogLine{
		Time:  b.opts.Now(),
		Level: level,
		Text:  fmt.Sprintf(format, args...),
	})

	if len(b.messages) > maxMessages {
		b.messages = b.messages[len(b.messages)-maxMessages:]
	}

	b.changed |= clabinternaltuibackend.TopicMessages
}

func (b *Backend) lab(name string) *clabinternaltuibackend.Lab {
	for i := range b.labs {
		if b.labs[i].Name == name {
			return &b.labs[i]
		}
	}

	return nil
}

func (b *Backend) specIndex(name string) int {
	return slices.IndexFunc(b.specs, func(s labSpec) bool { return s.name == name })
}

// Inspect implements clabinternaltuibackend.Backend.
func (b *Backend) Inspect(_ context.Context, ref clabinternaltuibackend.NodeRef) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lab := b.lab(ref.Lab)
	if lab == nil || lab.Node(ref.Name) == nil {
		return "", fmt.Errorf("node %q not found in lab %q", ref.Name, ref.Lab)
	}

	n := lab.Node(ref.Name)

	var s strings.Builder

	fmt.Fprintf(&s, "name:        %s\n", n.Container)
	fmt.Fprintf(&s, "id:          %s\n", n.ID)
	fmt.Fprintf(&s, "lab:         %s\n", n.Lab)
	fmt.Fprintf(&s, "kind:        %s\n", n.Kind)
	fmt.Fprintf(&s, "type:        %s\n", n.Type)
	fmt.Fprintf(&s, "group:       %s\n", n.Group)
	fmt.Fprintf(&s, "image:       %s\n", n.Image)
	fmt.Fprintf(&s, "state:       %s\n", n.State)
	fmt.Fprintf(&s, "status:      %s\n", n.Status)
	fmt.Fprintf(&s, "ipv4:        %s\n", n.IPv4)
	fmt.Fprintf(&s, "ipv6:        %s\n", n.IPv6)
	fmt.Fprintf(&s, "pid:         %d\n", n.Pid)
	fmt.Fprintf(&s, "owner:       %s\n", n.Owner)
	s.WriteString("labels:\n")

	keys := make([]string, 0, len(n.Labels))
	for k := range n.Labels {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	for _, k := range keys {
		fmt.Fprintf(&s, "  %s: %s\n", k, n.Labels[k])
	}

	return s.String(), nil
}

// ConnectMethods implements clabinternaltuibackend.Backend.
func (*Backend) ConnectMethods(
	node *clabinternaltuibackend.Node,
) []clabinternaltuibackend.ConnectMethod {
	shell := []string{"/bin/sh"}

	switch node.Kind {
	case kindSRL:
		shell = []string{"/opt/srlinux/bin/sr_cli"}
	case kindCEOS:
		shell = []string{"/usr/bin/Cli", "-p", "15"}
	}

	return clabinternaltuibackend.DefaultConnectMethods(shell, node.Kind)
}

// Interactive implements clabinternaltuibackend.Backend. There is no node to connect to,
// so the session only tells what would have been run.
func (b *Backend) Interactive(
	node clabinternaltuibackend.NodeRef, method clabinternaltuibackend.ConnectMethod,
) (*exec.Cmd, error) {
	what := "docker exec -it " + node.Container + " " + strings.Join(method.Shell, " ")
	if method.SSH {
		what = "ssh " + node.Container
	}

	b.mu.Lock()
	b.sessions = append(b.sessions, what)
	b.mu.Unlock()

	const script = `printf 'demo mode: this would run\n\n  %s\n\n' "$1"; ` +
		`printf 'press enter to return '; read -r _`

	return exec.CommandContext(
		context.Background(),
		"sh",
		"-c",
		script,
		"sh",
		what,
	), nil //nolint:gosec
}

// Sessions returns the commands of the interactive sessions that were asked for, in order.
func (b *Backend) Sessions() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return slices.Clone(b.sessions)
}

// Logs implements clabinternaltuibackend.Backend.
func (b *Backend) Logs(
	ctx context.Context,
	node clabinternaltuibackend.NodeRef,
	opts clabinternaltuibackend.LogOptions,
) (clabinternaltuibackend.LogStream, error) {
	b.mu.Lock()
	now := b.opts.Now()
	live := b.opts.Live
	b.mu.Unlock()

	const backlog = 240

	count := backlog
	if opts.Tail >= 0 && opts.Tail < count {
		count = opts.Tail
	}

	lines := make([]clabinternaltuibackend.LogLine, 0, count)
	for i := backlog - count; i < backlog; i++ {
		lines = append(
			lines,
			demoLogLine(node.Name, i, now.Add(-time.Duration(backlog-i)*time.Second)),
		)
	}

	ctx, cancel := context.WithCancel(ctx)

	s := &logStream{ch: make(chan []clabinternaltuibackend.LogLine, 1), cancel: cancel}
	s.ch <- lines

	if !opts.Follow || !live {
		if !opts.Follow {
			close(s.ch)
		}

		return s, nil
	}

	go func() {
		defer close(s.ch)

		const pace = 700 * time.Millisecond

		ticker := time.NewTicker(pace)
		defer ticker.Stop()

		for i := backlog; ; i++ {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				select {
				case s.ch <- []clabinternaltuibackend.LogLine{demoLogLine(node.Name, i, t)}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return s, nil
}

type logStream struct {
	ch     chan []clabinternaltuibackend.LogLine
	cancel context.CancelFunc
}

func (s *logStream) Lines() <-chan []clabinternaltuibackend.LogLine { return s.ch }
func (*logStream) Err() error                                       { return nil }
func (s *logStream) Close()                                         { s.cancel() }

var demoLogTemplates = []struct {
	level  string
	stderr bool
	text   string
}{
	{"INFO", false, "[INFO ] sr_linux_mgr: chassis manager ready type=7220-IXR-D3L"},
	{
		"INFO",
		false,
		"[INFO ] sr_bgp_mgr: neighbor 10.0.0.%d state=established peer-as=65000 group=spines",
	},
	{"DEBUG", false, "[DEBUG] sr_fib_mgr: programmed %d routes table=default took=12ms"},
	{
		"WARN",
		false,
		"[WARN ] sr_xdp_lc_1: port ethernet-1/50 oper-state=down reason=port-admin-disabled",
	},
	{
		"INFO",
		false,
		"[INFO ] sr_lldp_mgr: neighbor discovered interface=ethernet-1/%d system=spine1",
	},
	{
		"ERROR",
		true,
		"[ERROR] sr_bgp_mgr: neighbor 10.0.0.%d state=idle last-error=\"connection refused\"",
	},
	{"INFO", false, "[INFO ] sr_aaa_mgr: user admin authenticated from 172.20.20.1 service=ssh"},
	{"INFO", false, "[INFO ] sr_mgmt_server: commit confirmed id=%d by=admin"},
}

func demoLogLine(node string, i int, at time.Time) clabinternaltuibackend.LogLine {
	// Errors are rarer than the rest, so the log does not look like a failing node.
	pick := (i*7 + len(node)) % len(demoLogTemplates)
	if demoLogTemplates[pick].stderr && i%5 != 0 {
		pick = 0
	}

	tpl := demoLogTemplates[pick]
	text := tpl.text

	if strings.Contains(text, "%d") {
		text = fmt.Sprintf(text, 1+i%9)
	}

	return clabinternaltuibackend.LogLine{Time: at, Stderr: tpl.stderr, Text: text}
}

// Exec implements clabinternaltuibackend.Backend.
func (b *Backend) Exec(
	ctx context.Context, nodes []clabinternaltuibackend.NodeRef, command string,
) (<-chan clabinternaltuibackend.ExecResult, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("no command given")
	}

	b.mu.Lock()
	live := b.opts.Live
	b.mu.Unlock()

	out := make(chan clabinternaltuibackend.ExecResult, len(nodes))

	go func() {
		defer close(out)

		for i, n := range nodes {
			if live {
				const pace = 250 * time.Millisecond

				select {
				case <-ctx.Done():
					return
				case <-time.After(pace):
				}
			}

			res := clabinternaltuibackend.ExecResult{
				Node:     n,
				Stdout:   fmt.Sprintf("demo output of %q on %s\n", command, n.Name),
				Duration: time.Duration(40+i*13) * time.Millisecond, //nolint:mnd // made up
			}

			if strings.Contains(command, "fail") {
				res.ExitCode = 1
				res.Stdout = ""
				res.Stderr = "sh: " + command + ": not found\n"
			}

			out <- res
		}
	}()

	return out, nil
}

// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"slices"
	"sync"
	"time"

	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	// seriesLen is the number of samples kept per value.
	seriesLen = 60
	// sampleInterval is the pace the lab on screen is sampled at while rates are shown.
	sampleInterval = time.Second
	// fullSampleEvery makes every n-th round sample all labs.
	fullSampleEvery = 2

	bitsPerByte   = 8
	usecPerSecond = 1e6
	percent       = 100
)

// sampleTarget is a running container to sample.
type sampleTarget struct {
	container string
	id        string
	pid       int
}

// ifaceSample is the sampled state of an interface.
type ifaceSample struct {
	counters netCounters
	// hasRates is set once two samples allowed to compute the rates.
	hasRates                   bool
	rxBps, txBps, rxPps, txPps float64
	rxSeries, txSeries         []float64
}

// nodeSample is the sampled state of a container.
type nodeSample struct {
	id  string
	pid int

	cgroup string
	// cpuAt and cpuUsec are the previous reading of the CPU time, zero when there is none.
	cpuAt   time.Time
	cpuUsec uint64

	hasStats   bool
	cpuPercent float64
	memBytes   uint64
	memLimit   uint64
	cpuSeries  []float64

	// netAt is when the interface counters were read last.
	netAt              time.Time
	rxBps, txBps       float64
	rxSeries, txSeries []float64
	ifaces             map[string]*ifaceSample
}

// reading is what one sampling round read for a container.
type reading struct {
	dev     map[string]netCounters
	devOK   bool
	cgroup  string
	usage   cgroupUsage
	usageOK bool
}

// sampler tracks the resource usage and the traffic of the running containers. It reads
// files under /proc and /sys/fs/cgroup only, which needs no privileges.
type sampler struct {
	// root is where the proc and sys file systems are found, "/" outside of tests.
	root string

	mu    sync.RWMutex
	nodes map[string]*nodeSample
}

func newSampler(root string) *sampler {
	return &sampler{root: root, nodes: map[string]*nodeSample{}}
}

// sample takes a sample of every target and reports whether the set of interfaces changed.
func (s *sampler) sample(now time.Time, targets []sampleTarget) bool {
	changed := false

	for i := range targets {
		r := s.read(&targets[i])

		s.mu.Lock()
		changed = s.apply(now, &targets[i], &r) || changed
		s.mu.Unlock()
	}

	return changed
}

// read does the file reads of a sample. It runs without the lock held.
func (s *sampler) read(t *sampleTarget) reading {
	var r reading

	if dev, err := readNetDev(s.root, t.pid); err == nil {
		r.dev, r.devOK = dev, true
	}

	r.cgroup = s.cgroupOf(t)
	if r.cgroup == "" {
		return r
	}

	if usage, err := readCgroupUsage(s.root, r.cgroup); err == nil {
		r.usage, r.usageOK = usage, true
	}

	return r
}

// cgroupOf returns the cgroup of a container. It is looked up once per container start.
func (s *sampler) cgroupOf(t *sampleTarget) string {
	s.mu.RLock()
	node := s.nodes[t.container]
	known := node != nil && node.id == t.id && node.pid == t.pid && node.cgroup != ""

	var cgroup string
	if known {
		cgroup = node.cgroup
	}

	s.mu.RUnlock()

	if known {
		return cgroup
	}

	cgroup, err := readCgroupPath(s.root, t.pid, t.id)
	if err != nil {
		return ""
	}

	return cgroup
}

// apply stores a reading. The lock must be held.
func (s *sampler) apply(now time.Time, t *sampleTarget, r *reading) bool {
	changed := false

	node := s.nodes[t.container]
	if node == nil || node.id != t.id || node.pid != t.pid {
		// A new or restarted container starts counting from zero.
		node = &nodeSample{id: t.id, pid: t.pid, ifaces: map[string]*ifaceSample{}}
		s.nodes[t.container] = node
		changed = true
	}

	node.cgroup = r.cgroup
	node.applyUsage(now, r)

	if r.devOK {
		changed = node.applyNetDev(now, r.dev) || changed
	}

	return changed
}

func (n *nodeSample) applyUsage(now time.Time, r *reading) {
	if !r.usageOK {
		n.hasStats = false
		n.cpuAt = time.Time{}

		return
	}

	elapsed := now.Sub(n.cpuAt).Seconds()
	if !n.cpuAt.IsZero() && elapsed > 0 && r.usage.cpuUsec >= n.cpuUsec {
		// The share of one CPU: a container that keeps two cores busy shows 200.
		used := float64(r.usage.cpuUsec - n.cpuUsec)
		n.cpuPercent = used / (elapsed * usecPerSecond) * percent
		n.cpuSeries = pushSample(n.cpuSeries, n.cpuPercent)
	}

	n.cpuAt = now
	n.cpuUsec = r.usage.cpuUsec
	n.memBytes = r.usage.memBytes
	n.memLimit = r.usage.memLimit
	n.hasStats = true
}

// applyNetDev updates the interfaces with new counters and reports whether an interface
// appeared or went away.
func (n *nodeSample) applyNetDev(now time.Time, dev map[string]netCounters) bool {
	changed := false
	elapsed := now.Sub(n.netAt).Seconds()
	rated := false

	var rx, tx float64

	for name, counters := range dev {
		iface := n.ifaces[name]

		switch {
		case iface == nil:
			iface = &ifaceSample{}
			n.ifaces[name] = iface
			changed = true
		case !n.netAt.IsZero() && elapsed > 0:
			iface.rate(&counters, elapsed)
		}

		iface.counters = counters

		if iface.hasRates {
			rx += iface.rxBps
			tx += iface.txBps
			rated = true
		}
	}

	for name := range n.ifaces {
		if _, ok := dev[name]; !ok {
			delete(n.ifaces, name)

			changed = true
		}
	}

	if rated {
		n.rxBps, n.txBps = rx, tx
		n.rxSeries = pushSample(n.rxSeries, rx)
		n.txSeries = pushSample(n.txSeries, tx)
	}

	n.netAt = now

	return changed
}

// rate computes the rates from the growth of the counters over the elapsed seconds.
func (i *ifaceSample) rate(current *netCounters, elapsed float64) {
	previous := &i.counters

	i.rxBps = float64(counterDelta(previous.rxBytes, current.rxBytes)) * bitsPerByte / elapsed
	i.txBps = float64(counterDelta(previous.txBytes, current.txBytes)) * bitsPerByte / elapsed
	i.rxPps = float64(counterDelta(previous.rxPackets, current.rxPackets)) / elapsed
	i.txPps = float64(counterDelta(previous.txPackets, current.txPackets)) / elapsed
	i.hasRates = true
	i.rxSeries = pushSample(i.rxSeries, i.rxBps)
	i.txSeries = pushSample(i.txSeries, i.txBps)
}

// pushSample appends a value to a series and drops the oldest one beyond seriesLen.
func pushSample(series []float64, v float64) []float64 {
	if len(series) >= seriesLen {
		series = series[:copy(series, series[len(series)-seriesLen+1:])]
	}

	return append(series, v)
}

// retain forgets the containers that are not in the given set of running ones.
func (s *sampler) retain(running map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for name := range s.nodes {
		if _, ok := running[name]; !ok {
			delete(s.nodes, name)
		}
	}
}

// fill sets the sampled values of a node.
func (s *sampler) fill(node *clabinternaltuibackend.Node) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sample := s.nodes[node.Container]
	if sample == nil {
		return
	}

	node.HasStats = sample.hasStats
	node.CPUPercent = sample.cpuPercent
	node.MemBytes = sample.memBytes
	node.MemLimit = sample.memLimit
	node.RxBps = sample.rxBps
	node.TxBps = sample.txBps
	node.CPUSeries = slices.Clone(sample.cpuSeries)
	node.RxSeries = slices.Clone(sample.rxSeries)
	node.TxSeries = slices.Clone(sample.txSeries)
}

// interfaces returns the sampled interfaces of a container, keyed by their name.
func (s *sampler) interfaces(container string) map[string]clabinternaltuibackend.Interface {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sample := s.nodes[container]
	if sample == nil {
		return nil
	}

	out := make(map[string]clabinternaltuibackend.Interface, len(sample.ifaces))

	for name, iface := range sample.ifaces {
		out[name] = clabinternaltuibackend.Interface{
			Name:      name,
			HasStats:  true,
			RxBps:     iface.rxBps,
			TxBps:     iface.txBps,
			RxPps:     iface.rxPps,
			TxPps:     iface.txPps,
			RxBytes:   iface.counters.rxBytes,
			TxBytes:   iface.counters.txBytes,
			RxPackets: iface.counters.rxPackets,
			TxPackets: iface.counters.txPackets,
			RxSeries:  slices.Clone(iface.rxSeries),
			TxSeries:  slices.Clone(iface.txSeries),
		}
	}

	return out
}

// sampleLoop samples the running containers until ctx ends: all of them every other round,
// and the lab on screen in between while its traffic rates are shown.
func (b *Backend) sampleLoop(ctx context.Context) {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for round := 0; ; round++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		b.sampleOnce(round%fullSampleEvery == 0)
	}
}

// sampleOnce runs one sampling round. With nothing deployed there is nothing to read and
// the round is skipped.
func (b *Backend) sampleOnce(all bool) {
	lab := ""

	if !all {
		focus := b.currentFocus()
		if !focus.Rates || focus.Lab == "" {
			return
		}

		lab = focus.Lab
	}

	targets := b.inv.running(lab)
	if len(targets) == 0 {
		return
	}

	topics := clabinternaltuibackend.TopicStats
	if b.stats.sample(b.clock.Now(), targets) {
		topics |= clabinternaltuibackend.TopicInterfaces
	}

	b.mark(topics)
}

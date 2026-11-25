// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package fake

import (
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	mebibyte = 1 << 20

	// Traffic wanders around its starting rate within these bounds.
	rateFloor   = 0.35
	rateCeiling = 2.5
	// reverseShare is the traffic of the return direction relative to the forward one.
	reverseShare = 0.93
	// cpuPerMbit is the load a megabit per second of traffic adds to a node, in percent.
	cpuPerMbit = 0.35
)

// push returns a copy of series with v appended, keeping the most recent samples. A copy
// is made so that slices handed out earlier stay untouched.
func push(series []float64, v float64) []float64 {
	if len(series) >= seriesLen {
		series = series[len(series)-seriesLen+1:]
	}

	out := make([]float64, 0, seriesLen)
	out = append(out, series...)

	return append(out, v)
}

// jitter returns a random factor between low and high.
func (b *Backend) jitter(low, high float64) float64 {
	return low + (high-low)*b.rng.Float64()
}

// simulate advances the traffic and the load of the running nodes by one step. The lock
// must be held.
func (b *Backend) simulate() {
	b.simulateLinks()

	for i := range b.labs {
		lab := &b.labs[i]
		if !lab.Deployed {
			continue
		}

		for n := range lab.Nodes {
			b.simulateNode(&lab.Nodes[n])
		}
	}
}

func (b *Backend) simulateLinks() {
	for _, l := range b.links {
		lab := b.lab(l.lab)
		if lab == nil {
			continue
		}

		a, z := lab.Node(l.spec.a), lab.Node(l.spec.z)
		active := !l.spec.down && l.spec.rate > 0 && a != nil && z != nil &&
			a.Readiness() == clabinternaltuibackend.Ready &&
			z.Readiness() == clabinternaltuibackend.Ready

		if !active {
			l.ab, l.ba = 0, 0

			continue
		}

		l.ab = b.walk(l.ab, l.spec.rate)
		l.ba = b.walk(l.ba, l.spec.rate*reverseShare)
	}
}

func (b *Backend) walk(current, base float64) float64 {
	if current <= 0 {
		current = base
	}

	current *= b.jitter(0.82, 1.2) //nolint:mnd // drift a little each step

	return min(base*rateCeiling, max(base*rateFloor, current))
}

func (b *Backend) simulateNode(n *clabinternaltuibackend.Node) {
	if n.State != stateRunning {
		n.HasStats = false

		return
	}

	list := b.ifaces[n.Container]
	updated := make([]clabinternaltuibackend.Interface, len(list))
	copy(updated, list)

	var rx, tx float64

	for i := range updated {
		iface := &updated[i]
		iface.RxBps, iface.TxBps = b.interfaceRates(iface)
		iface.RxPps = iface.RxBps / (bitsPerByte * bytesPerPacket)
		iface.TxPps = iface.TxBps / (bitsPerByte * bytesPerPacket)
		iface.RxBytes += uint64(iface.RxBps / bitsPerByte)
		iface.TxBytes += uint64(iface.TxBps / bitsPerByte)
		iface.RxPackets += uint64(iface.RxPps)
		iface.TxPackets += uint64(iface.TxPps)
		iface.RxSeries = push(iface.RxSeries, iface.RxBps)
		iface.TxSeries = push(iface.TxSeries, iface.TxBps)
		iface.HasStats = true

		rx += iface.RxBps
		tx += iface.TxBps
	}

	b.ifaces[n.Container] = updated

	n.HasStats = true
	n.RxBps, n.TxBps = rx, tx
	n.CPUPercent = b.cpuLoad(n, rx+tx)
	n.MemBytes = uint64(baseMemory(n.Kind) * b.jitter(0.99, 1.01)) //nolint:mnd // nearly flat
	n.CPUSeries = push(n.CPUSeries, n.CPUPercent)
	n.RxSeries = push(n.RxSeries, rx)
	n.TxSeries = push(n.TxSeries, tx)
}

// interfaceRates returns the receive and transmit rates of an interface.
func (b *Backend) interfaceRates(iface *clabinternaltuibackend.Interface) (rx, tx float64) {
	if iface.Peer == nil {
		// The management interface only carries a trickle of traffic.
		return b.jitter(2e3, 6e3), b.jitter(3e3, 9e3) //nolint:mnd // a few kbit/s
	}

	for _, l := range b.links {
		if l.lab != iface.Node.Lab {
			continue
		}

		switch {
		case l.spec.a == iface.Node.Name && l.spec.aif == iface.Name:
			return l.ba, l.ab
		case l.spec.z == iface.Node.Name && l.spec.zif == iface.Name:
			return l.ab, l.ba
		}
	}

	return 0, 0
}

func (b *Backend) cpuLoad(n *clabinternaltuibackend.Node, bps float64) float64 {
	if n.Readiness() == clabinternaltuibackend.Starting {
		return b.jitter(120, 165) //nolint:mnd // a booting node is busy
	}

	const megabit = 1e6

	return (baseCPU(n.Kind) + bps/megabit*cpuPerMbit) * b.jitter(0.8, 1.25) //nolint:mnd
}

//nolint:mnd // made-up figures of the simulation
func baseCPU(kind string) float64 {
	switch kind {
	case kindSRL:
		return 9
	case kindSRSim:
		return 38
	case kindCEOS, kindIOL:
		return 14
	case kindCRPD:
		return 4
	case kindFortigate:
		return 22
	default:
		return 0.6
	}
}

//nolint:mnd // made-up figures of the simulation
func baseMemory(kind string) float64 {
	switch kind {
	case kindSRL:
		return 705 * mebibyte
	case kindSRSim:
		return 3277 * mebibyte
	case kindCEOS:
		return 1126 * mebibyte
	case kindIOL:
		return 512 * mebibyte
	case kindCRPD:
		return 388 * mebibyte
	case kindFortigate:
		return 2048 * mebibyte
	default:
		return 24 * mebibyte
	}
}

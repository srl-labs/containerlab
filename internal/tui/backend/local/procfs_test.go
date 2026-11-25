// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
)

const (
	testContainerID = "dc19dfceaf75ec03b5f65d473f08203a85b69ce1e163efcaa33e55d992fde006"
	testScope       = "/system.slice/docker-" + testContainerID + ".scope"

	netDevHeader = "Inter-|   Receive                                                |  " +
		"Transmit\n face |bytes    packets errs drop fifo frame compressed multicast|bytes" +
		"    packets errs drop fifo colls carrier compressed\n"
)

// netDevLine renders the line of an interface in /proc/<pid>/net/dev.
func netDevLine(name string, rxBytes, rxPackets, txBytes, txPackets uint64) string {
	return fmt.Sprintf("%6s: %7d %7d    0    0    0     0          0         0 %8d %7d"+
		"    0    0    0     0       0          0\n", name, rxBytes, rxPackets, txBytes, txPackets)
}

func TestParseNetDev(t *testing.T) {
	data := netDevHeader +
		netDevLine("lo", 6782013485, 24134026, 6782013485, 24134026) +
		netDevLine("eth0", 722633, 3776, 126, 3) +
		// A counter wide enough to touch the colon.
		"  e1-1:18446744073709551615 1 0 0 0 0 0 0 2 3 0 0 0 0 0 0\n" +
		netDevLine("ethernet-1/1.0", 10, 20, 30, 40) +
		"broken: 1 2 3\n" +
		"\n"

	want := map[string]netCounters{
		"eth0":           {rxBytes: 722633, rxPackets: 3776, txBytes: 126, txPackets: 3},
		"e1-1":           {rxBytes: 18446744073709551615, rxPackets: 1, txBytes: 2, txPackets: 3},
		"ethernet-1/1.0": {rxBytes: 10, rxPackets: 20, txBytes: 30, txPackets: 40},
	}

	got := parseNetDev([]byte(data))
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(netCounters{})); diff != "" {
		t.Errorf("unexpected counters (-want +got):\n%s", diff)
	}
}

func TestCounterDelta(t *testing.T) {
	tests := []struct {
		previous, current, want uint64
	}{
		{100, 150, 50},
		{100, 100, 0},
		// The counter was reset: what it shows was counted since.
		{1000, 40, 40},
		{0, 0, 0},
	}

	for _, tt := range tests {
		if got := counterDelta(tt.previous, tt.current); got != tt.want {
			t.Errorf("counterDelta(%d, %d) = %d, want %d", tt.previous, tt.current, got, tt.want)
		}
	}
}

func TestParseCgroupPath(t *testing.T) {
	const podmanID = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

	tests := map[string]struct {
		content string
		id      string
		want    string
		ok      bool
	}{
		"docker with the systemd driver": {
			content: "0::" + testScope + "\n",
			id:      testContainerID,
			want:    testScope,
			ok:      true,
		},
		"systemd runs in the container": {
			content: "0::" + testScope + "/init.scope\n",
			id:      testContainerID,
			want:    testScope,
			ok:      true,
		},
		"a deeper cgroup of the container": {
			content: "0::" + testScope + "/system.slice/sshd.service\n",
			id:      testContainerID,
			want:    testScope,
			ok:      true,
		},
		"docker with the cgroupfs driver": {
			content: "0::/docker/" + testContainerID + "\n",
			id:      testContainerID,
			want:    "/docker/" + testContainerID,
			ok:      true,
		},
		"podman": {
			content: "0::/machine.slice/libpod-" + podmanID + ".scope/container\n",
			id:      podmanID,
			want:    "/machine.slice/libpod-" + podmanID + ".scope",
			ok:      true,
		},
		"a path that does not name the container is used as it is": {
			content: "0::/kubepods/pod1234/abcd\n",
			id:      testContainerID,
			want:    "/kubepods/pod1234/abcd",
			ok:      true,
		},
		"hybrid hierarchy": {
			content: "12:pids:/docker/x\n1:name=systemd:/docker/x\n0::" + testScope + "\n",
			id:      testContainerID,
			want:    testScope,
			ok:      true,
		},
		"the path cannot leave the cgroup mount": {
			content: "0::/../../etc\n",
			want:    "/etc",
			ok:      true,
		},
		"cgroup v1 only":  {content: "12:pids:/docker/x\n11:memory:/docker/x\n"},
		"the root cgroup": {content: "0::/\n"},
		"empty":           {},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, ok := parseCgroupPath(tt.content, tt.id)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// fakeContainer writes the files a running container shows under /proc and the cgroup
// file system, below root.
type fakeContainer struct {
	t    *testing.T
	root string
	pid  int
	// cgroup is the cgroup of the container, procCgroup the one of its first process.
	cgroup, procCgroup string
}

func (c *fakeContainer) proc(name string) string {
	return filepath.Join(c.root, "proc", fmt.Sprint(c.pid), name)
}

func (c *fakeContainer) cg(name string) string {
	return filepath.Join(c.root, "sys", "fs", "cgroup", c.cgroup, name)
}

func (c *fakeContainer) setNetDev(lines ...string) {
	writeFile(c.t, c.proc("net/dev"), netDevHeader+strings.Join(lines, ""))
}

func (c *fakeContainer) setUsage(cpuUsec, memCurrent, inactiveFile uint64, memMax string) {
	writeFile(c.t, c.proc("cgroup"), "0::"+c.procCgroup+"\n")
	writeFile(c.t, c.cg("cpu.stat"),
		fmt.Sprintf("usage_usec %d\nuser_usec 1\nsystem_usec 2\nnr_periods 0\n", cpuUsec))
	writeFile(c.t, c.cg("memory.current"), fmt.Sprintf("%d\n", memCurrent))
	writeFile(c.t, c.cg("memory.stat"),
		fmt.Sprintf("anon 1\nfile 2\ninactive_file %d\nactive_file 3\n", inactiveFile))
	writeFile(c.t, c.cg("memory.max"), memMax+"\n")
}

func newFakeContainer(t *testing.T, root string) *fakeContainer {
	t.Helper()

	return &fakeContainer{
		t: t, root: root, pid: 4242,
		cgroup:     testScope,
		procCgroup: testScope + "/init.scope",
	}
}

func (c *fakeContainer) target() sampleTarget {
	return sampleTarget{container: "clab-lab1-leaf1", id: testContainerID, pid: c.pid}
}

func TestReadCgroupUsage(t *testing.T) {
	root := t.TempDir()
	c := newFakeContainer(t, root)
	c.setUsage(2500000, 300<<20, 100<<20, "1073741824")

	// The first process of the container sits one level below the container's cgroup.
	path, err := readCgroupPath(root, c.pid, testContainerID)
	if err != nil || path != testScope {
		t.Fatalf("readCgroupPath() = %q, %v, want %q", path, err, testScope)
	}

	got, err := readCgroupUsage(root, path)
	if err != nil {
		t.Fatal(err)
	}

	want := cgroupUsage{cpuUsec: 2500000, memBytes: 200 << 20, memLimit: 1 << 30}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Without a limit, and with more page cache than memory in use, which must not wrap.
	c.setUsage(1, 50, 80, "max")

	got, err = readCgroupUsage(root, path)
	if want := (cgroupUsage{cpuUsec: 1, memBytes: 50}); err != nil || got != want {
		t.Errorf("got %+v, %v, want %+v", got, err, want)
	}

	if _, err := readCgroupUsage(root, "/system.slice/other.scope"); err == nil {
		t.Error("reading a cgroup that does not exist did not fail")
	}

	if _, err := readCgroupPath(root, 1, testContainerID); err == nil {
		t.Error("reading the cgroup of a process that does not exist did not fail")
	}
}

func TestSamplerRates(t *testing.T) {
	root := t.TempDir()
	c := newFakeContainer(t, root)
	s := newSampler(root)
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	targets := []sampleTarget{c.target()}
	node := func() clabinternaltuibackend.Node {
		n := clabinternaltuibackend.Node{}
		n.Container = "clab-lab1-leaf1"
		s.fill(n.Container, &n)

		return n
	}

	c.setUsage(1_000_000, 300<<20, 100<<20, "max")
	c.setNetDev(netDevLine("lo", 5, 5, 5, 5), netDevLine("eth0", 1000, 10, 2000, 20),
		netDevLine("e1-1", 0, 0, 0, 0))

	if changed := s.sample(now, targets); !changed {
		t.Error("the first sample did not report new interfaces")
	}

	// One sample gives the usage and the counters, rates need a second one.
	first := node()
	if !first.HasStats || first.MemBytes != 200<<20 || first.MemLimit != 0 ||
		first.CPUPercent != 0 || first.RxBps != 0 || len(first.CPUSeries) != 0 {
		t.Errorf("unexpected node after the first sample: %+v", first)
	}

	ifaces := s.interfaces("clab-lab1-leaf1")
	if len(ifaces) != 2 || !ifaces["eth0"].HasStats || ifaces["eth0"].RxBytes != 1000 ||
		ifaces["eth0"].RxBps != 0 {
		t.Errorf("unexpected interfaces after the first sample: %+v", ifaces)
	}

	// Two seconds later: half a CPU was used, 1000 bytes came in on eth0.
	now = now.Add(2 * time.Second)

	c.setUsage(2_000_000, 300<<20, 100<<20, "max")
	c.setNetDev(netDevLine("eth0", 2000, 30, 2500, 24), netDevLine("e1-1", 500, 2, 0, 0))

	if changed := s.sample(now, targets); changed {
		t.Error("the second sample reported a change of the interfaces")
	}

	second := node()
	if second.CPUPercent != 50 || second.RxBps != 6000 || second.TxBps != 2000 {
		t.Errorf("cpu=%v rx=%v tx=%v, want 50, 6000, 2000", second.CPUPercent, second.RxBps,
			second.TxBps)
	}

	if diff := cmp.Diff([]float64{50}, second.CPUSeries); diff != "" {
		t.Errorf("unexpected CPU series (-want +got):\n%s", diff)
	}

	eth0 := s.interfaces("clab-lab1-leaf1")["eth0"]
	if eth0.RxBps != 4000 || eth0.TxBps != 2000 || eth0.RxPps != 10 || eth0.TxPps != 2 ||
		eth0.RxPackets != 30 || eth0.TxBytes != 2500 {
		t.Errorf("unexpected eth0: %+v", eth0)
	}

	// The counters of eth0 start over, as they do when an interface is created anew.
	now = now.Add(time.Second)

	c.setNetDev(netDevLine("eth0", 300, 3, 100, 1), netDevLine("e1-1", 500, 2, 0, 0))
	s.sample(now, targets)

	eth0 = s.interfaces("clab-lab1-leaf1")["eth0"]
	if eth0.RxBps != 2400 || eth0.TxBps != 800 || eth0.RxPps != 3 {
		t.Errorf("after a counter reset: %+v", eth0)
	}

	if diff := cmp.Diff([]float64{4000, 2400}, eth0.RxSeries); diff != "" {
		t.Errorf("unexpected series of eth0 (-want +got):\n%s", diff)
	}

	// An interface that went away is dropped, a new one is reported.
	now = now.Add(time.Second)

	c.setNetDev(netDevLine("eth0", 300, 3, 100, 1), netDevLine("e1-2", 1, 1, 1, 1))

	if changed := s.sample(now, targets); !changed {
		t.Error("a changed set of interfaces was not reported")
	}

	ifaces = s.interfaces("clab-lab1-leaf1")
	if _, gone := ifaces["e1-1"]; gone || len(ifaces) != 2 {
		t.Errorf("unexpected interfaces: %+v", ifaces)
	}
}

func TestSamplerKeepsBoundedSeries(t *testing.T) {
	root := t.TempDir()
	c := newFakeContainer(t, root)
	s := newSampler(root)
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	const samples = seriesLen + 15

	for i := range samples {
		c.setUsage(uint64(i)*1_000_000, 1, 0, "max")
		c.setNetDev(netDevLine("eth0", uint64(i*i)*125, 0, 0, 0))
		s.sample(now.Add(time.Duration(i)*time.Second), []sampleTarget{c.target()})
	}

	var node clabinternaltuibackend.Node

	node.Container = "clab-lab1-leaf1"
	s.fill(node.Container, &node)

	if len(node.CPUSeries) != seriesLen || len(node.RxSeries) != seriesLen ||
		len(node.TxSeries) != seriesLen {
		t.Fatalf("series lengths: cpu=%d rx=%d tx=%d, want %d", len(node.CPUSeries),
			len(node.RxSeries), len(node.TxSeries), seriesLen)
	}

	// The byte counter grows by (2i-1)*125 in second i, which is (2i-1) kbit/s. The oldest
	// samples are gone, the newest one comes last.
	last := float64(2*(samples-1)-1) * 1000
	oldest := float64(2*(samples-seriesLen)-1) * 1000

	if node.RxSeries[0] != oldest || node.RxSeries[seriesLen-1] != last || node.RxBps != last {
		t.Errorf("series runs from %v to %v, want %v to %v", node.RxSeries[0],
			node.RxSeries[seriesLen-1], oldest, last)
	}

	// The copy handed out does not change with later samples.
	kept := node.RxSeries[0]

	c.setNetDev(netDevLine("eth0", 0, 0, 0, 0))
	s.sample(now.Add(time.Hour), []sampleTarget{c.target()})

	if node.RxSeries[0] != kept {
		t.Error("a series handed out changed afterwards")
	}
}

func TestSamplerWithoutCgroupAccess(t *testing.T) {
	root := t.TempDir()
	c := newFakeContainer(t, root)
	s := newSampler(root)
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	// Only the interface counters are readable.
	c.setNetDev(netDevLine("eth0", 1000, 10, 0, 0))
	s.sample(now, []sampleTarget{c.target()})
	c.setNetDev(netDevLine("eth0", 2000, 20, 0, 0))
	s.sample(now.Add(time.Second), []sampleTarget{c.target()})

	var node clabinternaltuibackend.Node

	node.Container = "clab-lab1-leaf1"
	s.fill(node.Container, &node)

	if node.HasStats || node.CPUPercent != 0 || node.MemBytes != 0 {
		t.Errorf("usage reported without access to the cgroup: %+v", node)
	}

	if node.RxBps != 8000 {
		t.Errorf("RxBps = %v, want 8000", node.RxBps)
	}
}

func TestSamplerRestartedContainer(t *testing.T) {
	root := t.TempDir()
	c := newFakeContainer(t, root)
	s := newSampler(root)
	now := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)

	c.setUsage(9_000_000, 10, 0, "max")
	c.setNetDev(netDevLine("eth0", 1_000_000, 10, 0, 0))
	s.sample(now, []sampleTarget{c.target()})
	s.sample(now.Add(time.Second), []sampleTarget{c.target()})

	// The container comes back as another process, with counters starting at zero.
	c.pid = 5000
	c.setUsage(1000, 10, 0, "max")
	c.setNetDev(netDevLine("eth0", 10, 1, 0, 0))

	if changed := s.sample(now.Add(2*time.Second), []sampleTarget{c.target()}); !changed {
		t.Error("the restart was not reported as a change")
	}

	var node clabinternaltuibackend.Node

	node.Container = "clab-lab1-leaf1"
	s.fill(node.Container, &node)

	if node.CPUPercent != 0 || node.RxBps != 0 || len(node.RxSeries) != 0 {
		t.Errorf("values of the previous run were carried over: %+v", node)
	}

	// Containers that are gone are forgotten.
	s.retain(map[string]struct{}{})

	if got := s.interfaces("clab-lab1-leaf1"); got != nil {
		t.Errorf("a forgotten container still has interfaces: %+v", got)
	}
}

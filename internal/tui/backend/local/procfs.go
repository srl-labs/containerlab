// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const loopbackInterface = "lo"

// netCounters are the traffic counters of an interface.
type netCounters struct {
	rxBytes, rxPackets uint64
	txBytes, txPackets uint64
}

// readNetDev reads the interface counters of the network namespace the process lives in.
// The file is readable without privileges, unlike the namespace itself.
func readNetDev(root string, pid int) (map[string]netCounters, error) {
	data, err := os.ReadFile(filepath.Join(root, "proc", strconv.Itoa(pid), "net", "dev"))
	if err != nil {
		return nil, err
	}

	return parseNetDev(data), nil
}

// parseNetDev parses the interface table of /proc/<pid>/net/dev. The loopback interface is
// left out.
func parseNetDev(data []byte) map[string]netCounters {
	const (
		// Positions of the counters among the columns that follow the interface name.
		rxBytesCol, rxPacketsCol = 0, 1
		txBytesCol, txPacketsCol = 8, 9
	)

	counters := map[string]netCounters{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		// The two header lines have no colon, the columns of an interface follow one.
		name, values, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}

		name = strings.TrimSpace(name)
		fields := strings.Fields(values)

		if name == "" || name == loopbackInterface || len(fields) <= txPacketsCol {
			continue
		}

		counters[name] = netCounters{
			rxBytes:   parseCounter(fields[rxBytesCol]),
			rxPackets: parseCounter(fields[rxPacketsCol]),
			txBytes:   parseCounter(fields[txBytesCol]),
			txPackets: parseCounter(fields[txPacketsCol]),
		}
	}

	return counters
}

func parseCounter(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)

	return v
}

// counterDelta returns how much a counter grew. A counter that went backwards was reset, in
// which case its value is all that was counted since.
func counterDelta(previous, current uint64) uint64 {
	if current >= previous {
		return current - previous
	}

	return current
}

// readCgroupPath returns the cgroup of a container, relative to the cgroup mount, found
// through one of its processes.
func readCgroupPath(root string, pid int, containerID string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "proc", strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return "", err
	}

	path, ok := parseCgroupPath(string(data), containerID)
	if !ok {
		return "", errors.New("no cgroup v2 path of the container found")
	}

	return path, nil
}

// parseCgroupPath extracts the cgroup v2 path from the content of /proc/<pid>/cgroup and
// cuts it at the element that carries the container ID. The first process of a container
// that runs systemd sits in a child cgroup, ie. docker-<id>.scope/init.scope, while the
// accounting of the whole container is one level up.
func parseCgroupPath(content, containerID string) (string, bool) {
	for line := range strings.SplitSeq(content, "\n") {
		// The unified hierarchy is the entry with ID 0 and no controllers.
		path, found := strings.CutPrefix(line, "0::")
		if !found {
			continue
		}

		path = filepath.Clean("/" + strings.TrimSpace(path))
		if path == "/" {
			// The root cgroup accounts for the whole host.
			return "", false
		}

		return trimToContainer(path, containerID), true
	}

	return "", false
}

func trimToContainer(path, containerID string) string {
	if containerID == "" {
		return path
	}

	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i > 0; i-- {
		if strings.Contains(parts[i], containerID) {
			return strings.Join(parts[:i+1], "/")
		}
	}

	return path
}

// cgroupUsage is the resource usage read from a cgroup.
type cgroupUsage struct {
	// cpuUsec is the CPU time consumed so far, in microseconds.
	cpuUsec uint64
	// memBytes is the memory in use without the page cache that can be dropped.
	memBytes uint64
	// memLimit is the memory limit, zero when there is none.
	memLimit uint64
}

// readCgroupUsage reads the CPU and memory accounting of a cgroup v2 directory.
func readCgroupUsage(root, cgroup string) (cgroupUsage, error) {
	dir := filepath.Join(root, "sys", "fs", "cgroup", cgroup)

	var usage cgroupUsage

	cpuStat, err := os.ReadFile(filepath.Join(dir, "cpu.stat"))
	if err != nil {
		return usage, err
	}

	cpuUsec, ok := statValue(cpuStat, "usage_usec")
	if !ok {
		return usage, fmt.Errorf("no usage_usec in %s", filepath.Join(dir, "cpu.stat"))
	}

	usage.cpuUsec = cpuUsec

	current, err := os.ReadFile(filepath.Join(dir, "memory.current"))
	if err != nil {
		return usage, err
	}

	usage.memBytes = parseCounter(strings.TrimSpace(string(current)))

	// The inactive page cache is reclaimable and not what a user calls used memory. The
	// container runtimes subtract it as well.
	if memStat, err := os.ReadFile(filepath.Join(dir, "memory.stat")); err == nil {
		if inactive, ok := statValue(memStat, "inactive_file"); ok && inactive < usage.memBytes {
			usage.memBytes -= inactive
		}
	}

	// A limit of "max" does not parse and leaves the limit at zero.
	if limit, err := os.ReadFile(filepath.Join(dir, "memory.max")); err == nil {
		usage.memLimit = parseCounter(strings.TrimSpace(string(limit)))
	}

	return usage, nil
}

// statValue finds a key in a file of "key value" lines.
func statValue(data []byte, key string) (uint64, bool) {
	for line := range strings.SplitSeq(string(data), "\n") {
		name, value, found := strings.Cut(line, " ")
		if !found || name != key {
			continue
		}

		v, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)

		return v, err == nil
	}

	return 0, false
}

// Package velocpu provides a process-wide allocator that hands out host CPUs to
// VeloCloud nodes (cvce/cvcg) so they share the host predictably.
//
// Both the Edge (cvce) and Gateway (cvcg) kinds draw from the same pool. Each
// node either pins an explicit cpu-set (reserved out of the pool) or claims a
// block of CPUs. The allocator first hands out dedicated blocks until the pool
// is exhausted, then stacks further nodes onto the least-occupied block
// (uniform fill) up to the oversubscription factor, and caps each node with a
// hard CPU quota of block-size/factor so one node cannot steal cycles from
// another sharing the same CPUs.
//
// The pool defaults to every logical CPU on the host and can be narrowed with
// CLAB_VELO_CPU_POOL (a cpu-set list, e.g. "2-15"). The oversubscription factor
// is set with CLAB_VELO_CPU_OVERSUBSCRIBE (a positive integer, default 2).
package velocpu

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	// EnvPool overrides the set of host CPUs velo nodes may be pinned to.
	EnvPool = "CLAB_VELO_CPU_POOL"
	// EnvOversubscribe sets how many nodes may share one CPU block.
	EnvOversubscribe = "CLAB_VELO_CPU_OVERSUBSCRIBE"
)

// Allocation is the result of a Claim: the cpu-set a node is pinned to and the
// hard CPU quota (in cores) it is limited to.
type Allocation struct {
	CPUSet   string
	CPUQuota float64
}

// block is a set of CPUs shared by up to `factor` nodes.
type block struct {
	cpus  []int
	set   string
	count int
}

var (
	mu       sync.Mutex
	free     []int        // sorted, still-available CPUs in the pool
	reserved map[int]bool // CPUs already handed out or explicitly reserved
	blocks   []*block     // allocated blocks, in creation order
	factor   int          // oversubscription factor (>= 1)
	inited   bool
)

// Reset clears allocator state. Intended for tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	free = nil
	reserved = nil
	blocks = nil
	factor = 0
	inited = false
}

func ensurePool() error {
	if inited {
		return nil
	}

	var pool []int
	if v := os.Getenv(EnvPool); v != "" {
		cpus, err := parseCPUSet(v)
		if err != nil {
			return fmt.Errorf("invalid %s=%q: %w", EnvPool, v, err)
		}
		pool = cpus
	} else {
		for i := 0; i < runtime.NumCPU(); i++ {
			pool = append(pool, i)
		}
	}

	factor = 2
	if v := os.Getenv(EnvOversubscribe); v != "" {
		f, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || f < 1 {
			return fmt.Errorf("invalid %s=%q: want a positive integer", EnvOversubscribe, v)
		}
		factor = f
	}

	reserved = make(map[int]bool, len(pool))
	free = pool
	sort.Ints(free)
	inited = true

	return nil
}

// Reserve removes an explicit cpu-set from the pool so the allocator won't hand
// the same CPUs to another velo node. It errors if any CPU is already taken
// (two nodes overlapping) or falls outside the pool. Explicit cpu-sets are not
// oversubscribed; the node keeps whatever cpu limit it was given.
func Reserve(cpuset string) error {
	mu.Lock()
	defer mu.Unlock()

	if err := ensurePool(); err != nil {
		return err
	}

	cpus, err := parseCPUSet(cpuset)
	if err != nil {
		return fmt.Errorf("invalid cpu-set %q: %w", cpuset, err)
	}

	inPool := make(map[int]bool, len(free))
	for _, c := range free {
		inPool[c] = true
	}

	for _, c := range cpus {
		if reserved[c] {
			return fmt.Errorf("cpu %d is assigned to more than one velo node (cpu-set %q "+
				"overlaps another node's CPUs); pin explicit velo nodes to the high end of the "+
				"pool, or set cpu-set on all of them", c, cpuset)
		}
		if !inPool[c] {
			return fmt.Errorf("cpu %d (from cpu-set %q) is not in the velo CPU pool; widen %s",
				c, cpuset, EnvPool)
		}
	}

	for _, c := range cpus {
		reserved[c] = true
	}
	free = withoutCPUs(free, cpus)

	return nil
}

// Claim places a node needing `cores` CPUs onto a block and returns its cpu-set
// and hard CPU quota. A dedicated block is opened while the pool has room;
// once the pool is exhausted the node is stacked onto the least-occupied block
// of the same size (uniform fill), up to the oversubscription factor.
func Claim(cores int) (Allocation, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := ensurePool(); err != nil {
		return Allocation{}, err
	}

	if cores <= 0 {
		return Allocation{}, fmt.Errorf("velo cpu count must be positive, got %d", cores)
	}

	b, err := pickBlock(cores)
	if err != nil {
		return Allocation{}, err
	}
	b.count++

	return Allocation{
		CPUSet:   b.set,
		CPUQuota: float64(cores) / float64(factor),
	}, nil
}

// pickBlock returns the block a new node of the given size should join, opening
// a fresh dedicated block if the pool still has room, otherwise the
// least-occupied existing block that is below the oversubscription factor.
func pickBlock(cores int) (*block, error) {
	if len(free) >= cores {
		cpus := append([]int(nil), free[:cores]...)
		free = free[cores:]
		for _, c := range cpus {
			reserved[c] = true
		}
		b := &block{cpus: cpus, set: formatCPUSet(cpus)}
		blocks = append(blocks, b)
		return b, nil
	}

	var best *block
	for _, b := range blocks {
		if len(b.cpus) != cores || b.count >= factor {
			continue
		}
		if best == nil || b.count < best.count {
			best = b
		}
	}
	if best != nil {
		return best, nil
	}

	return nil, fmt.Errorf("velo CPU pool exhausted: cannot place a node needing %d CPUs at "+
		"oversubscription %d; widen %s or raise %s", cores, factor, EnvPool, EnvOversubscribe)
}

// withoutCPUs returns the sorted slice with the given CPUs removed.
func withoutCPUs(in, remove []int) []int {
	drop := make(map[int]bool, len(remove))
	for _, c := range remove {
		drop[c] = true
	}
	out := in[:0:0]
	for _, c := range in {
		if !drop[c] {
			out = append(out, c)
		}
	}
	return out
}

// parseCPUSet parses a Linux cpu-set list like "0-3,5,7-8" into a sorted,
// de-duplicated slice of CPU numbers.
func parseCPUSet(s string) ([]int, error) {
	seen := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			start, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return nil, fmt.Errorf("bad cpu range %q", part)
			}
			end, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return nil, fmt.Errorf("bad cpu range %q", part)
			}
			if end < start {
				return nil, fmt.Errorf("bad cpu range %q: end < start", part)
			}
			for c := start; c <= end; c++ {
				seen[c] = true
			}
			continue
		}
		c, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("bad cpu %q", part)
		}
		seen[c] = true
	}

	if len(seen) == 0 {
		return nil, fmt.Errorf("empty cpu-set")
	}

	cpus := make([]int, 0, len(seen))
	for c := range seen {
		if c < 0 {
			return nil, fmt.Errorf("negative cpu %d", c)
		}
		cpus = append(cpus, c)
	}
	sort.Ints(cpus)

	return cpus, nil
}

// formatCPUSet renders a sorted slice of CPUs as a compact cpu-set list,
// collapsing runs into ranges (e.g. [2,3,4,7] -> "2-4,7").
func formatCPUSet(cpus []int) string {
	if len(cpus) == 0 {
		return ""
	}

	var b strings.Builder
	start := cpus[0]
	prev := cpus[0]

	flush := func(lo, hi int) {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		if lo == hi {
			b.WriteString(strconv.Itoa(lo))
		} else {
			b.WriteString(strconv.Itoa(lo))
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(hi))
		}
	}

	for _, c := range cpus[1:] {
		if c == prev+1 {
			prev = c
			continue
		}
		flush(start, prev)
		start, prev = c, c
	}
	flush(start, prev)

	return b.String()
}

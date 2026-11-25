package virt

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/mackerelio/go-osstat/memory"
)

type MemoryType int

const (
	MemoryTypeTotal MemoryType = iota
	MemoryTypeAvailable
)

// GetSysMemory reports on total installed or available memory (in bytes).
func GetSysMemory(ctx context.Context, mt MemoryType) uint64 {
	memoryResult, err := memory.Get()
	if err != nil {
		log.FromContext(ctx).Errorf("unable to determine available memory: %v", err)
		return 0
	}
	switch mt {
	case MemoryTypeAvailable:
		return memoryResult.Available
	case MemoryTypeTotal:
		return memoryResult.Total
	}
	return 0
}

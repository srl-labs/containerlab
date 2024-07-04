package types

import (
	"fmt"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/virt"
)

type HostRequirements struct {
	SSSE3 bool `json:"ssse3,omitempty"` // ssse3 cpu instruction
	// indicates that KVM virtualization is required for this node to run
	VirtRequired bool `json:"virt-required,omitempty"`
	// the minimum amount of vcpus this node requires
	MinVCPU           int           `json:"min-vcpu,omitempty"`
	MinVCPUFailAction FailBehaviour `json:"min-vcpu-fail-action,omitempty"`
	// The minimum amount of memory this node requires
	MinAvailMemoryGb           int           `json:"min-free-memory,omitempty"`
	MinAvailMemoryGbFailAction FailBehaviour `json:"min-free-memory-fail-action,omitempty"`
}

type FailBehaviour int

const (
	FailBehaviourLog FailBehaviour = iota
	FailBehaviourError
)

// NewHostRequirements is the constructor for new HostRequirements structs.
func NewHostRequirements() *HostRequirements {
	return &HostRequirements{
		MinVCPUFailAction:          FailBehaviourLog,
		MinAvailMemoryGbFailAction: FailBehaviourLog,
	}
}

// Verify runs verification checks against the host requirements set for a node.
func (h *HostRequirements) Verify(kindName, nodeName string) error {
	// check virtualization Support
	if h.VirtRequired && !virt.VerifyVirtSupport() {
		return fmt.Errorf("CPU virtualization support is required for node %q (%s)", nodeName, kindName)
	}
	// check SSSE3 support on amd64 arch only as it is an x86_64 instruction
	if runtime.GOARCH == "amd64" && h.SSSE3 && !virt.VerifySSSE3Support() {
		return fmt.Errorf("SSSE3 CPU feature is required for node %q (%s)", nodeName, kindName)
	}
	// check minimum vCPUs
	if valid, num := h.verifyMinVCpu(); !valid {
		message := fmt.Sprintf("node %q (%s) requires minimum %d vCPUs, but the host only has %d vCPUs", nodeName, kindName, h.MinVCPU, num)
		switch h.MinAvailMemoryGbFailAction {
		case FailBehaviourError:
			return fmt.Errorf(message)
		case FailBehaviourLog:
			log.Error(message)
		}
	}
	// check minimum FreeMemory
	if valid, num := h.verifyMinAvailMemory(); !valid {
		message := fmt.Sprintf("node %q (%s) has a minimum available memory requirement of %d GB whilst only %d GB memory is available",
			nodeName, kindName, h.MinAvailMemoryGb, num)
		switch h.MinAvailMemoryGbFailAction {
		case FailBehaviourError:
			return fmt.Errorf(message)
		case FailBehaviourLog:
			log.Error(message)
		}
	}
	return nil
}

// verifyMinAvailMemory verifies that the node requirement for minimum free memory is met.
// It returns a bool indicating if the requirement is met and the amount of available memory in GB.
func (h *HostRequirements) verifyMinAvailMemory() (bool, uint64) {
	availMemGB := virt.GetSysMemory(virt.MemoryTypeAvailable) / 1024 / 1024 / 1024

	// if the MinFreeMemory amount is 0, there is no requirement defined, so result is true
	if h.MinAvailMemoryGb == 0 {
		return true, availMemGB
	}

	// amount of Free Memory must be greater-equal the requirement
	result := uint64(h.MinAvailMemoryGb) <= availMemGB
	return result, availMemGB
}

// verifyMinVCpu verifies that the node requirement for minimum vCPU count is met.
func (h *HostRequirements) verifyMinVCpu() (bool, int) {
	numCpu := runtime.NumCPU()

	// if the minCPU amount is 0, there is no requirement defined, so result is true
	if h.MinVCPU == 0 {
		return true, numCpu
	}

	// count of vCPUs must be greater-equal the requirement
	result := h.MinVCPU <= numCpu
	return result, numCpu
}

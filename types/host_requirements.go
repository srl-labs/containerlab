package types

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/utils"
	"github.com/srl-labs/containerlab/virt"
)

type HostRequirements struct {
	SSSE3                     bool          `json:"ssse3,omitempty"`         // ssse3 cpu instruction
	VirtRequired              bool          `json:"virt-required,omitempty"` // indicates that KVM virtualization is required for this node to run
	MinVCPU                   int           `json:"min-vcpu,omitempty"`      // the minimum amount of vcpus this node requires
	MinVCPUFailAction         FailBehaviour `json:"min-vcpu-fail-action,omitempty"`
	MinFreeMemoryGb           int           `json:"min-free-memory,omitempty"` // The minimum amount of memory this node requires
	MinFreeMemoryGbFailAction FailBehaviour `json:"min-free-memory-fail-action,omitempty"`
}

type FailBehaviour int

const (
	FailBehaviourLog FailBehaviour = iota
	FailBehaviourError
)

// NewHostRequirements is the constructor for new HostRequirements structs
func NewHostRequirements() *HostRequirements {
	return &HostRequirements{
		MinVCPUFailAction:         FailBehaviourLog,
		MinFreeMemoryGbFailAction: FailBehaviourLog,
	}
}

func (h *HostRequirements) Verify() error {
	// check virtualization Support
	if h.VirtRequired && !virt.VerifyVirtSupport() {
		return fmt.Errorf("the CPU virtualization support is required, but not available")
	}
	// check SSSE3 support
	if h.SSSE3 && !virt.VerifySSSE3Support() {
		return fmt.Errorf("the SSSE3 CPU feature is required, but not available")
	}
	// check minimum vCPUs
	if valid, num := h.verifyMinVCpu(); !valid {
		message := fmt.Sprintf("the defined minimum vCPU amount based on the nodes in your topology is %d whilst only %d vCPUs are available", h.MinVCPU, num)
		switch h.MinFreeMemoryGbFailAction {
		case FailBehaviourError:
			return fmt.Errorf(message)
		case FailBehaviourLog:
			log.Error(message)
		default:
			log.Error(message)
		}
	}
	// check minimum FreeMemory
	if valid, num := h.verifyMinFreeMemory(); !valid {
		message := fmt.Sprintf("the defined minimum free memory based on the nodes in your topology is %d GB whilst only %d GB memory is free", h.MinFreeMemoryGb, num)
		switch h.MinFreeMemoryGbFailAction {
		case FailBehaviourError:
			return fmt.Errorf(message)
		case FailBehaviourLog:
			log.Error(message)
		default:
			log.Error(message)
		}
	}

	return nil
}

// verifyMinFreeMemory verify that the amount of free memory with the requirement
// it returns a bool indicating if the requirement is satisfied and the amount of free memory in GB
func (h *HostRequirements) verifyMinFreeMemory() (bool, uint64) {
	// if the MinFreeMemory amount is 0, there is no requirement defined, so result is true
	// if != 0 then amount of Free Memory must be greater-equal the requirement
	freeMemG := virt.GetSysMemory(virt.MemoryTypeAvailable) / 1024 / 1024 / 1024

	boolResult := h.MinFreeMemoryGb == 0 || h.MinFreeMemoryGb != 0 && uint64(h.MinFreeMemoryGb) <= freeMemG
	return boolResult, freeMemG
}

// verifyMinVCpu verify that the amount of re
func (h *HostRequirements) verifyMinVCpu() (bool, int) {
	// if the minCPU amount is 0, there is no requirement defined, so result is true
	// if != 0 then amount of vCPUs must be greater-equal the requirement
	boolResult := h.MinVCPU == 0 || h.MinVCPU != 0 && h.MinVCPU <= runtime.NumCPU()
	return boolResult, runtime.NumCPU()
}

func DisableTxOffload(n *NodeConfig) error {
	// skip this if node runs in host mode
	if strings.ToLower(n.NetworkMode) == "host" {
		return nil
	}
	// disable tx checksum offload for linux containers on eth0 interfaces
	nodeNS, err := ns.GetNS(n.NSPath)
	if err != nil {
		return err
	}
	err = nodeNS.Do(func(_ ns.NetNS) error {
		// disabling offload on eth0 interface
		err := utils.EthtoolTXOff("eth0")
		if err != nil {
			log.Infof("Failed to disable TX checksum offload for 'eth0' interface for Linux '%s' node: %v", n.ShortName, err)
		}
		return err
	})
	return err
}

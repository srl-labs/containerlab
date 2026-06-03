package types

import (
	"github.com/charmbracelet/log"
	clabutils "github.com/srl-labs/containerlab/utils"
)

type TopologyDiffAction string

const (
	TopologyDiffActionNone     TopologyDiffAction = ""
	TopologyDiffActionRestart  TopologyDiffAction = "restart"
	TopologyDiffActionRecreate TopologyDiffAction = "recreate"
)

type TopologyDiff struct {
	// capture the fields that have changed
	Fields []string
}

// if there is a diff, returns true
func (d *TopologyDiff) HasDiff() bool {
	return d != nil && len(d.Fields) > 0
}

// fields that require a recreate of the container
var RecreateFields = map[string]bool{
	"Image":       true,
	"Type":        true,
	"Env":         true,
	"Cmd":         true,
	"Entrypoint":  true,
	"Runtime":     true,
	"NetworkMode": true,
	"CPU":         true,
	"CPUSet":      true,
	"Memory":      true,
	"Devices":     true,
	"CapAdd":      true,
	"ShmSize":     true,
	"User":        true,
	"Ports":       true,
	"Binds":       true,
	"License":     true,
}

// fields which just need a restart to reapply
var RestartFields = map[string]bool{
	"Exec": true,
}

// DefaultAction returns the action for this diff based on field categorization.
// Returns None if no action is needed (no diff or no matching fields).
func (d *TopologyDiff) DefaultAction() TopologyDiffAction {
	if d == nil || !d.HasDiff() {
		return TopologyDiffActionNone
	}

	needsRestart := false
	for _, field := range d.Fields {
		if RecreateFields[field] {
			return TopologyDiffActionRecreate
		}
		if RestartFields[field] {
			needsRestart = true
		} else if !RecreateFields[field] {
			log.Warnf("Field %q changed but is not supported for live update", field)
		}
	}

	if needsRestart {
		return TopologyDiffActionRestart
	}
	return TopologyDiffActionNone
}

func ComputeTopologyDiff(oldTopo, newTopo *Topology, nodeName string) *TopologyDiff {
	var fields []string

	if oldTopo == nil {
		return &TopologyDiff{}
	}

	if oldTopo.GetNodeType(nodeName) != newTopo.GetNodeType(nodeName) {
		fields = append(fields, "Type")
	}
	if oldTopo.GetNodeImage(nodeName) != newTopo.GetNodeImage(nodeName) {
		fields = append(fields, "Image")
	}
	if oldTopo.GetNodeEntrypoint(nodeName) != newTopo.GetNodeEntrypoint(nodeName) {
		fields = append(fields, "Entrypoint")
	}
	if oldTopo.GetNodeCmd(nodeName) != newTopo.GetNodeCmd(nodeName) {
		fields = append(fields, "Cmd")
	}
	if !clabutils.SlicesEqualOrBothEmpty(oldTopo.GetNodeExec(nodeName), newTopo.GetNodeExec(nodeName)) {
		fields = append(fields, "Exec")
	}
	if !clabutils.MapsEqualOrBothEmpty(oldTopo.GetNodeEnv(nodeName), newTopo.GetNodeEnv(nodeName)) {
		fields = append(fields, "Env")
	}
	oldBinds, _ := oldTopo.GetNodeBinds(nodeName)
	newBinds, _ := newTopo.GetNodeBinds(nodeName)
	if !clabutils.SlicesEqualOrBothEmpty(oldBinds, newBinds) {
		fields = append(fields, "Binds")
	}
	if !clabutils.SlicesEqualOrBothEmpty(oldTopo.GetNodeDevices(nodeName), newTopo.GetNodeDevices(nodeName)) {
		fields = append(fields, "Devices")
	}
	if !clabutils.SlicesEqualOrBothEmpty(oldTopo.GetNodeCapAdd(nodeName), newTopo.GetNodeCapAdd(nodeName)) {
		fields = append(fields, "CapAdd")
	}
	if oldTopo.GetNodeShmSize(nodeName) != newTopo.GetNodeShmSize(nodeName) {
		fields = append(fields, "ShmSize")
	}
	oldPorts, _, _ := oldTopo.GetNodePorts(nodeName)
	newPorts, _, _ := newTopo.GetNodePorts(nodeName)
	if !clabutils.PortSetsEqual(oldPorts, newPorts) {
		fields = append(fields, "Ports")
	}
	if oldTopo.GetNodeUser(nodeName) != newTopo.GetNodeUser(nodeName) {
		fields = append(fields, "User")
	}
	if oldTopo.GetNodeNetworkMode(nodeName) != newTopo.GetNodeNetworkMode(nodeName) {
		fields = append(fields, "NetworkMode")
	}
	if oldTopo.GetNodeRuntime(nodeName) != newTopo.GetNodeRuntime(nodeName) {
		fields = append(fields, "Runtime")
	}
	if oldTopo.GetNodeCPU(nodeName) != newTopo.GetNodeCPU(nodeName) {
		fields = append(fields, "CPU")
	}
	if oldTopo.GetNodeCPUSet(nodeName) != newTopo.GetNodeCPUSet(nodeName) {
		fields = append(fields, "CPUSet")
	}
	if oldTopo.GetNodeMemory(nodeName) != newTopo.GetNodeMemory(nodeName) {
		fields = append(fields, "Memory")
	}
	if oldTopo.GetNodeLicense(nodeName) != newTopo.GetNodeLicense(nodeName) {
		fields = append(fields, "License")
	}

	return &TopologyDiff{Fields: fields}
}

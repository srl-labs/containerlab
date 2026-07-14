package types

import (
	"github.com/charmbracelet/log"
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
	"Hostname":    true,
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
	"Components":  true,
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

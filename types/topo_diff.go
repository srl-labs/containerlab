package types

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

// DefaultAction conservatively recreates a node for every configuration change.
// Node kinds that can reconcile a field live may override the default in GetReconcilePlan.
func (d *TopologyDiff) DefaultAction() TopologyDiffAction {
	if !d.HasDiff() {
		return TopologyDiffActionNone
	}
	return TopologyDiffActionRecreate
}

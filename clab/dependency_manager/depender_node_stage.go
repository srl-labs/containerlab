package dependency_manager

import (
	"fmt"

	"github.com/srl-labs/containerlab/types"
)

// dependerNodeStage is used to keep track of waitgroups that should be decreased (unblocked)
// as soon as a certain delpoy stage is reached.
type dependerNodeStage struct {
	depender *DependencyNode    // reference to the node
	stage    types.WaitForStage // reference to the nodes wg that is to be decremented
}

func newDependerNodeStage(node *DependencyNode, stage types.WaitForStage) *dependerNodeStage {
	return &dependerNodeStage{
		depender: node,
		stage:    stage,
	}
}

func (d *dependerNodeStage) SignalDone() {
	d.depender.getStageWG(d.stage).Done()
}

func (d *dependerNodeStage) IncreaseDependencyWG() {
	d.depender.stageWG[d.stage].Add(1)
}

func (d *dependerNodeStage) String() string {
	return fmt.Sprintf("Node %s, State %s", d.depender.GetShortName(), d.stage)
}

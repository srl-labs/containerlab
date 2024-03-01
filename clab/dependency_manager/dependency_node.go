package dependency_manager

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/types"
)

// DependencyNode is the representation of a node in the dependency concept.
type DependencyNode struct {
	nodes.Node
	// stageWG is a map of waitgroup per a given stage.
	// When the waitgroup associated with the stage is Done, the node is considered to have reached the stage.
	stageWG map[types.WaitForStage]*sync.WaitGroup
	// mustWait determines if dependers exist for the given stages.
	mustWait map[types.WaitForStage]bool
	depender map[types.WaitForStage][]*dependerNodeStage

	m sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func NewDependencyNode(node nodes.Node) *DependencyNode {
	d := &DependencyNode{
		Node:     node,
		stageWG:  map[types.WaitForStage]*sync.WaitGroup{},
		mustWait: map[types.WaitForStage]bool{},
		depender: map[types.WaitForStage][]*dependerNodeStage{},
	}

	for _, p := range types.GetWaitForStages() {
		d.stageWG[p] = &sync.WaitGroup{}
		d.depender[p] = nil
	}

	// check if Stage based execs exist, if so make sure it will be
	// waited for the respective phases
	hasExecs := len(d.Config().Stages.CreateLinks.ExecContainerCommands) > 0
	if hasExecs {
		d.mustWait[types.WaitForCreateLinks] = true
	}

	hasExecs = len(d.Config().Stages.Configure.ExecContainerCommands) > 0
	if hasExecs {
		d.mustWait[types.WaitForConfigure] = true
	}

	hasExecs = len(d.Config().Stages.Healthy.ExecContainerCommands) > 0
	if hasExecs {
		d.mustWait[types.WaitForHealthy] = true
	}

	return d
}

// getStageWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *DependencyNode) getStageWG(n types.WaitForStage) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.stageWG[n]; !exists {
		d.stageWG[n] = &sync.WaitGroup{}
	}
	return d.stageWG[n]
}

// EnterStage is called by a node that is meant to enter the specified stage.
// The call will be blocked until all dependencies for the node to enter the stage are met.
func (d *DependencyNode) EnterStage(ctx context.Context, p types.WaitForStage) {
	log.Debugf("Stage Change: Enter Wait -> %s - %s", d.GetShortName(), p)
	d.stageWG[p].Wait()
	log.Debugf("Stage Change: Enter Go -> %s - %s", d.GetShortName(), p)

	var err error
	execs := []*exec.ExecCmd{}
	switch p {
	case types.WaitForCreate:
		execs, err = d.Config().Stages.Create.GetExecs()
	case types.WaitForCreateLinks:
		execs, err = d.Config().Stages.CreateLinks.GetExecs()
	case types.WaitForConfigure:
		execs, err = d.Config().Stages.Configure.GetExecs()
	case types.WaitForHealthy:
		execs, err = d.Config().Stages.Healthy.GetExecs()
	case types.WaitForExit:
		execs, err = d.Config().Stages.Exit.GetExecs()
	}
	if err != nil {
		log.Errorf("error running exec on %s: %v", d.GetShortName(), err)
	}
	// exec the commands
	execResultCollection := exec.NewExecCollection()

	for _, exec := range execs {
		execResult, err := d.RunExec(ctx, exec)
		if err != nil {
			log.Errorf("error on exec in node %s for stage %s: %v", d.GetShortName(), p, err)
		}
		execResultCollection.Add(d.GetShortName(), execResult)
	}
	execResultCollection.Log()
}

// Done is called by a node that has finished all tasks for the provided stage.
// The dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
func (d *DependencyNode) Done(p types.WaitForStage) {
	// iterate through all the dependers, that wait for the specific stage
	// and reduce the waitgroup
	log.Debugf("StateChange: Done -> %s - %s", d.GetShortName(), p)
	for _, depender := range d.depender[p] {
		log.Debugf("StateChange: Node %s unblocking %s", d.GetShortName(), depender.String())
		depender.SignalDone()
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *DependencyNode) AddDepender(dependerStage types.WaitForStage, depender *DependencyNode, stage types.WaitForStage) error {
	// Create a new DependerNodeStage
	dependerNS := newDependerNodeStage(depender, dependerStage)

	// adding the dependerNodeState entity to the specific state of d
	// this will result in a .Done() call when d finishes state
	d.depender[stage] = append(d.depender[stage], dependerNS)
	// for the depender to know how many dependencies to wait for,
	// the waitgroup need to be increaded by 1 as well
	dependerNS.IncreaseDependencyWG()
	d.mustWait[stage] = true

	return nil
}

// MustWait returns true if the node needs to wait for the given stage, because dependers do exist.
func (d *DependencyNode) MustWait(state types.WaitForStage) bool {
	if b, exists := d.mustWait[state]; exists {
		return b
	}
	return false
}

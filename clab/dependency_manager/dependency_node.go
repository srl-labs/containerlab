package dependency_manager

import (
	"context"
	"fmt"
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

	stage := d.Config().Stages

	// check if Stage based execs exist, if so make sure it will be
	// waited for the respective phases
	d.mustWait[types.WaitForCreateLinks] = d.mustWait[types.WaitForCreateLinks] || stage.CreateLinks.Execs.HasCommands()
	d.mustWait[types.WaitForConfigure] = d.mustWait[types.WaitForConfigure] || stage.Configure.Execs.HasCommands()
	d.mustWait[types.WaitForHealthy] = d.mustWait[types.WaitForHealthy] || stage.Healthy.Execs.HasCommands()

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

func (d *DependencyNode) getExecs(p types.WaitForStage, t types.CommandType) ([]*exec.ExecCmd, error) {
	switch p {
	case types.WaitForCreate:
		return d.Config().Stages.Create.GetExecCommands(t)
	case types.WaitForCreateLinks:
		return d.Config().Stages.CreateLinks.GetExecCommands(t)
	case types.WaitForConfigure:
		return d.Config().Stages.Configure.GetExecCommands(t)
	case types.WaitForHealthy:
		return d.Config().Stages.Healthy.GetExecCommands(t)
	case types.WaitForExit:
		return d.Config().Stages.Exit.GetExecCommands(t)
	}
	return nil, fmt.Errorf("stage %s unknown", p)
}

// EnterStage is called by a node that is meant to enter the specified stage.
// The call will be blocked until all dependencies for the node to enter the stage are met.
func (d *DependencyNode) EnterStage(ctx context.Context, p types.WaitForStage) {
	log.Debugf("Stage Change: Enter Wait -> %s - %s", d.GetShortName(), p)
	d.stageWG[p].Wait()
	log.Debugf("Stage Change: Enter Go -> %s - %s", d.GetShortName(), p)
	d.runExecs(ctx, types.CommandTypeEnter, p)
}

func (d *DependencyNode) runExecs(ctx context.Context, ct types.CommandType, p types.WaitForStage) {
	execs, err := d.getExecs(p, ct)
	if err != nil {
		log.Errorf("error getting exec commands defined for %s: %v", d.GetShortName(), err)
	}

	if len(execs) == 0 {
		return
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
func (d *DependencyNode) Done(ctx context.Context, p types.WaitForStage) {
	// iterate through all the dependers, that wait for the specific stage
	// and reduce the waitgroup
	d.runExecs(ctx, types.CommandTypeExit, p)
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

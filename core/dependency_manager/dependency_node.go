package dependency_manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodeshost "github.com/srl-labs/containerlab/nodes/host"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// DependencyNode is the representation of a node in the dependency concept.
type DependencyNode struct {
	clabnodes.Node
	// stageWG is a map of waitgroup per a given stage.
	// When the waitgroup associated with the stage is Done, the node is considered to have
	// reached the stage.
	stageWG map[clabtypes.WaitForStage]*sync.WaitGroup
	// mustWait determines if dependers exist for the given stages.
	mustWait map[clabtypes.WaitForStage]bool
	depender map[clabtypes.WaitForStage][]*dependerNodeStage

	m sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func NewDependencyNode(node clabnodes.Node) *DependencyNode {
	d := &DependencyNode{
		Node:     node,
		stageWG:  map[clabtypes.WaitForStage]*sync.WaitGroup{},
		mustWait: map[clabtypes.WaitForStage]bool{},
		depender: map[clabtypes.WaitForStage][]*dependerNodeStage{},
	}

	for _, p := range clabtypes.GetWaitForStages() {
		d.stageWG[p] = &sync.WaitGroup{}
		d.depender[p] = nil
	}

	stage := d.Config().Stages

	// check if Stage based execs exist, if so make sure it will be
	// waited for the respective phases
	d.mustWait[clabtypes.WaitForCreateLinks] = d.mustWait[clabtypes.WaitForCreateLinks] ||
		stage.CreateLinks.Execs.HasCommands()
	d.mustWait[clabtypes.WaitForConfigure] = d.mustWait[clabtypes.WaitForConfigure] ||
		stage.Configure.Execs.HasCommands()
	d.mustWait[clabtypes.WaitForHealthy] = d.mustWait[clabtypes.WaitForHealthy] ||
		stage.Healthy.Execs.HasCommands()

	return d
}

// getStageWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *DependencyNode) getStageWG(n clabtypes.WaitForStage) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.stageWG[n]; !exists {
		d.stageWG[n] = &sync.WaitGroup{}
	}

	return d.stageWG[n]
}

func (d *DependencyNode) getExecs(stage clabtypes.WaitForStage,
	execPhase clabtypes.ExecPhase,
) ([]*clabtypes.Exec, error) {
	var sb clabtypes.StageBase

	switch stage {
	case clabtypes.WaitForCreate:
		sb = d.Config().Stages.Create.StageBase
	case clabtypes.WaitForCreateLinks:
		sb = d.Config().Stages.CreateLinks.StageBase
	case clabtypes.WaitForConfigure:
		sb = d.Config().Stages.Configure.StageBase
	case clabtypes.WaitForHealthy:
		sb = d.Config().Stages.Healthy.StageBase
	case clabtypes.WaitForExit:
		sb = d.Config().Stages.Exit.StageBase
	default:
		return nil, fmt.Errorf("stage %s unknown", stage)
	}

	result := []*clabtypes.Exec{}

	for _, x := range sb.Execs {
		// filter the list of commands for the given phase (on-enter / on-exit)
		if x.Phase == execPhase {
			result = append(result, x)
		}
	}

	return result, nil
}

// EnterStage is called by a node that is meant to enter the specified stage.
// The call will be blocked until all dependencies for the node to enter the stage are met.
func (d *DependencyNode) EnterStage(ctx context.Context, p clabtypes.WaitForStage) {
	log.Debugf("Stage Change: Enter Wait -> %s - %s", d.GetShortName(), p)

	d.stageWG[p].Wait()

	log.Debugf("Stage Change: Enter Go -> %s - %s", d.GetShortName(), p)

	d.runExecs(ctx, clabtypes.CommandExecutionPhaseEnter, p)
}

func (d *DependencyNode) runExecs(
	ctx context.Context,
	execPhase clabtypes.ExecPhase,
	stage clabtypes.WaitForStage,
) {
	execs, err := d.getExecs(stage, execPhase)
	if err != nil {
		log.Errorf("error getting exec commands defined for %s: %v", d.GetShortName(), err)
	}

	if len(execs) == 0 {
		return
	}

	// exec the commands
	execResultCollection := clabexec.NewExecCollection()

	var execResult *clabexec.ExecResult

	var hostname string

	for _, exec := range execs {
		execCmd, err := exec.GetExecCmd()
		if err != nil {
			log.Errorf(
				"%s stage %s error parsing command: %s", d.GetShortName(), stage, exec.String(),
			)
		}

		switch exec.Target {
		case clabtypes.CommandTargetContainer:
			execResult, err = d.RunExec(ctx, execCmd)
			hostname = d.GetShortName()
		case clabtypes.CommandTargetHost:
			execResult, err = clabnodeshost.RunExec(ctx, execCmd)
			hostname = fmt.Sprintf("host via %s", d.GetShortName())
		default:
			continue
		}

		if err != nil {
			log.Errorf("error on exec in node %s for stage %s: %v", d.GetShortName(), stage, err)
		} else {
			execResultCollection.Add(hostname, execResult)
		}
	}

	execResultCollection.Log()
}

// Done is called by a node that has finished all tasks for the provided stage.
// The dependent nodes will be "notified" that an additional (if multiple exist) dependency is
// satisfied.
func (d *DependencyNode) Done(ctx context.Context, p clabtypes.WaitForStage) {
	// iterate through all the dependers, that wait for the specific stage
	// and reduce the waitgroup
	log.Debugf("StateChange: Done -> %s - %s", d.GetShortName(), p)

	d.runExecs(ctx, clabtypes.CommandExecutionPhaseExit, p)

	for _, depender := range d.depender[p] {
		log.Debugf("StateChange: Node %s unblocking %s", d.GetShortName(), depender.String())
		depender.SignalDone()
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the
// depender to increase the waitgroup count for the depender.
func (d *DependencyNode) AddDepender(dependerStage clabtypes.WaitForStage,
	depender *DependencyNode, stage clabtypes.WaitForStage,
) error {
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
func (d *DependencyNode) MustWait(state clabtypes.WaitForStage) bool {
	if b, exists := d.mustWait[state]; exists {
		return b
	}

	return false
}

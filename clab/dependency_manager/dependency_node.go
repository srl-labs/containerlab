package dependency_manager

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

// DependencyNode is the representation of a node in the dependency concept.
type DependencyNode struct {
	name string
	// stageWG is a map of waitgroup per a given stage.
	// When the waitgroup associated with the stage is Done, the node is considered to have reached the stage.
	stageWG map[types.WaitForStage]*sync.WaitGroup
	// mustWait determines if dependers exist for the given stages.
	mustWait map[types.WaitForStage]bool
	depender map[types.WaitForStage][]*dependerNodeStage

	m sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func newDependencyNode(name string) *DependencyNode {
	d := &DependencyNode{
		name:     name,
		stageWG:  map[types.WaitForStage]*sync.WaitGroup{},
		mustWait: map[types.WaitForStage]bool{},
		depender: map[types.WaitForStage][]*dependerNodeStage{},
	}

	for _, p := range types.GetWaitForStages() {
		d.stageWG[p] = &sync.WaitGroup{}
		d.depender[p] = nil
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
func (d *DependencyNode) EnterStage(p types.WaitForStage) {
	log.Debugf("Stage Change: Enter Wait -> %s - %s", d.name, p)
	d.stageWG[p].Wait()
	log.Debugf("Stage Change: Enter Go -> %s - %s", d.name, p)
}

// SignalDone is called by a node that has finished all tasks for the provided stage.
// The dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
func (d *DependencyNode) Done(p types.WaitForStage) {
	// iterate through all the dependers, that wait for the specific stage
	// and reduce the waitgroup
	log.Debugf("StateChange: Done -> %s - %s", d.name, p)
	for _, depender := range d.depender[p] {
		log.Debugf("StateChange: Node %s unblocking %s", d.name, depender.String())
		depender.SignalDone()
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *DependencyNode) addDepender(dependerStage types.WaitForStage, depender *DependencyNode, stage types.WaitForStage) error {
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
func (d *DependencyNode) MustWait(state types.WaitForStage) (bool, error) {
	if b, exists := d.mustWait[state]; exists {
		return b, nil
	}
	return false, fmt.Errorf("stage %q not found", d.name)
}

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
	return fmt.Sprintf("Node %s, State %s", d.depender.name, d.stage)
}

package dependency_manager

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

// dependencyNode is the representation of a node in the dependency concept.
type dependencyNode struct {
	name                string
	stageBarrier        map[types.WaitForStage]*sync.WaitGroup
	stageBarrierCounter map[types.WaitForStage]uint
	depender            map[types.WaitForStage][]*dependerNodeStage

	m sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func newDependencyNode(name string) *dependencyNode {
	d := &dependencyNode{
		name:                name,
		stageBarrier:        map[types.WaitForStage]*sync.WaitGroup{},
		stageBarrierCounter: map[types.WaitForStage]uint{},
		depender:            map[types.WaitForStage][]*dependerNodeStage{},
	}

	for _, p := range types.GetWaitForStages() {
		d.stageBarrier[p] = &sync.WaitGroup{}
		d.stageBarrierCounter[p] = 0
		d.depender[p] = nil
	}

	return d
}

// getStageWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *dependencyNode) getStageWG(n types.WaitForStage) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.stageBarrier[n]; !exists {
		d.stageBarrier[n] = &sync.WaitGroup{}
	}
	return d.stageBarrier[n]
}

func (d *dependencyNode) EnterStage(p types.WaitForStage) {
	log.Debugf("Stage Change: Enter Wait -> %s - %s", d.name, p)
	d.stageBarrier[p].Wait()
	log.Debugf("Stage Change: Enter Go -> %s - %s", d.name, p)
}

// Done indicates that the node has reached the given state.
// The waitgroup associated with this state will be Done as well.
func (d *dependencyNode) Done(p types.WaitForStage) {
	// iterate through all the dependers, that wait for the specific stage
	// and reduce the waitgroup
	log.Debugf("StateChange: Done -> %s - %s", d.name, p)
	for _, depender := range d.depender[p] {
		log.Debugf("StateChange: Node %s unblocking %s", d.name, depender.String())
		depender.SignalDone()
		d.stageBarrierCounter[p]--
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *dependencyNode) addDepender(dependerStage types.WaitForStage, depender *dependencyNode, stage types.WaitForStage) error {
	// Create a new DependerNodeStage
	dependerNS := newDependerNodeStage(depender, dependerStage)

	// adding the dependerNodeState entity to the specific state of d
	// this will result in a .Done() call when d finishes state
	d.depender[stage] = append(d.depender[stage], dependerNS)
	// for the depender to know how many dependencies to wait for,
	// the waitgroup need to be increaded by 1 as well
	dependerNS.IncreaseDependencyWG()
	d.stageBarrierCounter[stage]++

	return nil
}

func (d *dependencyNode) GetDependerCount(state types.WaitForStage) (uint, error) {
	if count, exists := d.stageBarrierCounter[state]; exists {
		return count, nil
	}
	return 0, fmt.Errorf("state %q not found", d.name)
}

// dependerNodeStage is used to keep track of waitgroups that should be decreased (unblocked)
// as soon as a certain delpoy stage is reached.
type dependerNodeStage struct {
	depender *dependencyNode    // reference to the node
	stage    types.WaitForStage // reference to the nodes wg that is to be decremented
}

func newDependerNodeStage(node *dependencyNode, stage types.WaitForStage) *dependerNodeStage {
	return &dependerNodeStage{
		depender: node,
		stage:    stage,
	}
}

func (d *dependerNodeStage) SignalDone() {
	d.depender.getStageWG(d.stage).Done()
}

func (d *dependerNodeStage) IncreaseDependencyWG() {
	d.depender.stageBarrier[d.stage].Add(1)
}

func (d *dependerNodeStage) String() string {
	return fmt.Sprintf("Node %s, State %s", d.depender.name, d.stage)
}

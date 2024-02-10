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
	phaseBarrier        map[types.WaitForPhase]*sync.WaitGroup
	phaseBarrierCounter map[types.WaitForPhase]uint
	depender            map[types.WaitForPhase][]*dependerNodePhase

	m sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func newDependencyNode(name string) *dependencyNode {
	d := &dependencyNode{
		name:                name,
		phaseBarrier:        map[types.WaitForPhase]*sync.WaitGroup{},
		phaseBarrierCounter: map[types.WaitForPhase]uint{},
		depender:            map[types.WaitForPhase][]*dependerNodePhase{},
	}

	for _, p := range types.GetWaitForPhases() {
		d.phaseBarrier[p] = &sync.WaitGroup{}
		d.phaseBarrierCounter[p] = 0
		d.depender[p] = nil
	}

	return d
}

// getPhaseWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *dependencyNode) getPhaseWG(n types.WaitForPhase) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.phaseBarrier[n]; !exists {
		d.phaseBarrier[n] = &sync.WaitGroup{}
	}
	return d.phaseBarrier[n]
}

func (d *dependencyNode) EnterPhase(p types.WaitForPhase) {
	log.Debugf("Phase Change: Enter Wait -> %s - %s", d.name, p)
	d.phaseBarrier[p].Wait()
	log.Debugf("Phase Change: Enter Go -> %s - %s", d.name, p)
}

// Done indicates that the node has reached the given state.
// The waitgroup associated with this state will be Done as well.
func (d *dependencyNode) Done(p types.WaitForPhase) {
	// iterate through all the dependers, that wait for the specific phase
	// and reduce the waitgroup
	log.Debugf("StateChange: Done -> %s - %s", d.name, p)
	for _, depender := range d.depender[p] {
		log.Debugf("StateChange: Node %s unblocking %s", d.name, depender.String())
		depender.SignalDone()
		d.phaseBarrierCounter[p]--
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *dependencyNode) addDepender(dependerPhase types.WaitForPhase, depender *dependencyNode, phase types.WaitForPhase) error {
	// Create a new DependerNodePhase
	dependerNS := newDependerNodePhase(depender, dependerPhase)

	// adding the dependerNodeState entity to the specific state of d
	// this will result in a .Done() call when d finishes state
	d.depender[phase] = append(d.depender[phase], dependerNS)
	// for the depender to know how many dependencies to wait for,
	// the waitgroup need to be increaded by 1 as well
	dependerNS.IncreaseDependencyWG()
	d.phaseBarrierCounter[phase]++

	return nil
}

func (d *dependencyNode) GetDependerCount(state types.WaitForPhase) (uint, error) {
	if count, exists := d.phaseBarrierCounter[state]; exists {
		return count, nil
	}
	return 0, fmt.Errorf("state %q not found", d.name)
}

// dependerNodePhase is used to keep track of waitgroups that should be decreased (unblocked)
// as soon as a certain delpoy stage is reached.
type dependerNodePhase struct {
	depender *dependencyNode    // reference to the node
	phase    types.WaitForPhase // reference to the nodes wg that is to be decremented
}

func newDependerNodePhase(node *dependencyNode, phase types.WaitForPhase) *dependerNodePhase {
	return &dependerNodePhase{
		depender: node,
		phase:    phase,
	}
}

func (d *dependerNodePhase) SignalDone() {
	d.depender.getPhaseWG(d.phase).Done()
}

func (d *dependerNodePhase) IncreaseDependencyWG() {
	d.depender.phaseBarrier[d.phase].Add(1)
}

func (d *dependerNodePhase) String() string {
	return fmt.Sprintf("Node %s, State %s", d.depender.name, d.phase)
}

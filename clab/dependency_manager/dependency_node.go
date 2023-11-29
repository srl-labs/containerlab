package dependency_manager

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

// dependencyNode is the representation of a node in the dependency concept.
type dependencyNode struct {
	name         string
	stateBarrier map[types.WaitForPhase]*sync.WaitGroup
	depender     map[types.WaitForPhase][]*dependerNodeState
	m            sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func newDependencyNode(name string) *dependencyNode {
	d := &dependencyNode{
		name: name,
		// WaitState is initialized with a wait group for each node state.
		// WaitState is used to for a dependee to wait for a depender to reach a certain state.
		stateBarrier: map[types.WaitForPhase]*sync.WaitGroup{},
		depender:     map[types.WaitForPhase][]*dependerNodeState{},
	}

	for _, p := range types.WaitForPhases {
		d.stateBarrier[p] = &sync.WaitGroup{}
		d.depender[p] = nil
	}

	return d
}

// getStateWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *dependencyNode) getStateWG(n types.WaitForPhase) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.stateBarrier[n]; !exists {
		d.stateBarrier[n] = &sync.WaitGroup{}
	}
	return d.stateBarrier[n]
}

func (d *dependencyNode) Enter(p types.WaitForPhase) {
	log.Debugf("StateChange: Enter Wait -> %s - %s", d.name, p)
	d.stateBarrier[p].Wait()
	log.Debugf("StateChange: Enter Go -> %s - %s", d.name, p)
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
	}
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *dependencyNode) addDepender(depender *dependencyNode, state types.WaitForPhase) error {
	// Create a new DependerNodeState
	dependerNS := NewDependerNodeState(depender, types.WaitForCreate)

	// adding the dependerNodeState entity to the specific state of d
	// this will result in a .Done() call when d finishes state
	d.depender[state] = append(d.depender[state], dependerNS)
	// for the depender to know how many dependencies to wait for,
	// the waitgroup need to be increaded by 1 as well
	dependerNS.IncreaseDependencyWG()

	return nil
}

type dependerNodeState struct {
	depender *dependencyNode
	state    types.WaitForPhase
}

func NewDependerNodeState(node *dependencyNode, state types.WaitForPhase) *dependerNodeState {
	return &dependerNodeState{
		depender: node,
		state:    state,
	}
}

func (d *dependerNodeState) SignalDone() {
	d.depender.getStateWG(d.state).Done()
}

func (d *dependerNodeState) IncreaseDependencyWG() {
	d.depender.stateBarrier[d.state].Add(1)
}

func (d *dependerNodeState) String() string {
	return fmt.Sprintf("Node %s, State %s", d.depender.name, d.state)
}

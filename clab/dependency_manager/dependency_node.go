package dependency_manager

import "sync"

// dependencyNode is the representation of a node in the dependency concept.
type dependencyNode struct {
	name          string
	WaitState     map[NodeState]*sync.WaitGroup
	nodeDependers map[string]*dependencyNode
	m             sync.Mutex
}

// newDependencyNode initializes a dependencyNode with the given name.
func newDependencyNode(name string) *dependencyNode {
	d := &dependencyNode{
		name: name,
		// WaitState is initialized with a wait group for each node state.
		// WaitState is used to for a dependee to wait for a depender to reach a certain state.
		WaitState:     map[NodeState]*sync.WaitGroup{},
		nodeDependers: map[string]*dependencyNode{},
	}

	// node states must be initialized,
	// because the wait groups are not allowed to become negative
	for _, s := range RegularNodeStates {
		d.getStateWG(s).Add(1)
	}

	return d
}

// getStateWG retrieves the provided node state waitgroup if it exists
// otherwise initializes it.
func (d *dependencyNode) getStateWG(n NodeState) *sync.WaitGroup {
	d.m.Lock()
	defer d.m.Unlock()

	if _, exists := d.WaitState[n]; !exists {
		d.WaitState[n] = &sync.WaitGroup{}
	}
	return d.WaitState[n]
}

// WaitFor makes the caller wait for the node to reach the provided NodeState.
func (d *dependencyNode) WaitFor(n NodeState) error {
	wg := d.getStateWG(n)
	wg.Wait()
	return nil
}

// Done indicates that the node has reached the given state.
// The waitgroup associated with this state will be Done as well.
func (d *dependencyNode) Done(n NodeState) error {
	wg := d.getStateWG(n)
	wg.Done()

	// special handling of dependencies
	if n == NodeStateCreated {
		for _, d := range d.nodeDependers {
			d.Done(dependency)
		}
	}
	return nil
}

// addDepender adds a depender to the dependencyNode. This will also add the dependee to the depender.
// to increase the waitgroup count for the depender.
func (d *dependencyNode) addDepender(depender *dependencyNode) error {
	d.nodeDependers[depender.name] = depender
	depender.addDependee()
	return nil
}

// addDependee is an internal call used to increase the Dependee WaitGroup.
func (d *dependencyNode) addDependee() {
	d.getStateWG(dependency).Add(1)
}

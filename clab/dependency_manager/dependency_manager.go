package dependency_manager

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type DependencyManager interface {
	// AddNode adds a node to the dependency manager.
	AddNode(name string)
	// AddDependency adds a dependency between depender and dependee.
	// The depender will effectively wait for the dependee to finish.
	AddDependency(dependee, depender string) error
	// WaitForNodeDependencies is called by a node that is meant to be created.
	// This call will bock until all the nodes that this node depends on are created.
	WaitForNodeDependencies(nodeName string) error
	// SignalDone is called by a node that has finished all tasks for the provided State.
	// internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
	SignalDone(nodeName string, state NodeState)
	// CheckAcyclicity checks if dependencies contain cycles.
	CheckAcyclicity() error
	// String returns a string representation of dependencies recorded with dependency manager.
	String() string
	// WaitForNodes blocks until all the nodes in the provided list have reached the provided state.
	WaitForNodes(nodeNames []string, status NodeState) error
}

type NodeState int

const (
	NodeStateCreated NodeState = iota
	// dependency is a special state that is used to indicate that a node depends on other node.
	dependency = 99
)

var RegularNodeStates = []NodeState{NodeStateCreated}

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

// defaultDependencyManager is the default implementation of the DependencyManager.
type defaultDependencyManager struct {
	nodes map[string]*dependencyNode
}

// NewDependencyManager constructor.
func NewDependencyManager() DependencyManager {
	return &defaultDependencyManager{
		nodes: map[string]*dependencyNode{},
	}
}

// AddNode adds a node to the dependency manager.
func (dm *defaultDependencyManager) AddNode(name string) {
	dm.nodes[name] = newDependencyNode(name)
}

// AddDependency adds a dependency between depender and dependee.
// The depender will effectively wait for the dependee to finish.
func (dm *defaultDependencyManager) AddDependency(dependee, depender string) error {
	// first check if the referenced nodes are known to the dm
	depder, exists := dm.nodes[depender]
	if !exists {
		return fmt.Errorf("node %q is not known to the dependency manager", depender)
	}
	depdee, exists := dm.nodes[dependee]
	if !exists {
		return fmt.Errorf("node %q is not known to the dependency manager", dependee)
	}

	depdee.addDepender(depder)
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

// WaitForNodeDependencies is called by a node that is meant to be created.
// This call will bock until all the nodes that this node depends on are created.
func (dm *defaultDependencyManager) WaitForNodeDependencies(nodeName string) error {
	// first check if the referenced node is known to the dm
	node, exists := dm.nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %q is not known to the dependency manager", nodeName)
	}
	node.WaitFor(dependency)
	return nil
}

// SignalDone is called by a node that has finished the indicated NodeState process and reached the desired state.
// Internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
func (dm *defaultDependencyManager) SignalDone(nodeName string, state NodeState) {
	// first check if the referenced node is known to the dm
	node, exists := dm.nodes[nodeName]
	if !exists {
		log.Errorf("tried to Signal Done for node %q but node is unknown to the DependencyManager", nodeName)
		return
	}
	node.Done(state)
}

func (dm *defaultDependencyManager) checkNodesExist(nodeNames []string) error {
	var missing []string
	for _, nodeName := range nodeNames {
		if _, exists := dm.nodes[nodeName]; !exists {
			missing = append(missing, nodeName)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("dependency manager has no notion of the following nodes %s", strings.Join(missing, ", "))
}

func (dm *defaultDependencyManager) WaitForNodes(nodeNames []string, state NodeState) error {
	err := dm.checkNodesExist(nodeNames)
	if err != nil {
		return err
	}
	for _, nodename := range nodeNames {
		dm.nodes[nodename].WaitFor(state)
	}
	return nil
}

// String returns a string representation of dependencies recorded with dependency manager.
func (dm *defaultDependencyManager) String() string {
	// since dm.nodeDependers contains a map of dependee->[dependers] it is not
	// particularly suitable for displaying the dependency graph
	// this function reverses the order so that it becomes depender->[dependees]

	// map to record the dependencies in string based representation
	dependencies := map[string][]string{}

	// prepare dependencies table
	for name := range dm.nodes {
		dependencies[name] = []string{}
	}

	// build the dependency datastruct
	for nodeName, node := range dm.nodes {
		for _, depender := range node.nodeDependers {
			dependencies[nodeName] = append(dependencies[nodeName], depender.name)
		}
	}

	var result []string
	// print dependencies
	for nodename, deps := range dependencies {
		result = append(result, fmt.Sprintf("%s -> [ %s ]", nodename, strings.Join(deps, ", ")))
	}
	return strings.Join(result, "\n")
}

// CheckAcyclicity checks if dependencies contain cycles.
func (dm *defaultDependencyManager) CheckAcyclicity() error {
	log.Debugf("Dependencies:\n%s", dm.String())

	nodeDependers := map[string][]string{}

	for _, n := range dm.nodes {
		nodeDependers[n.name] = []string{}
		for _, dep := range n.nodeDependers {
			nodeDependers[n.name] = append(nodeDependers[n.name], dep.name)
		}
	}

	if !isAcyclic(nodeDependers, 1) {
		return fmt.Errorf("cyclic dependencies found!\n%s", dm.String())
	}

	return nil
}

// isAcyclic checks the provided dependencies map for cycles.
// i indicates the check round. Must be set to 1.
func isAcyclic(nodeDependers map[string][]string, i int) bool {
	// no more nodes then the graph is acyclic
	if len(nodeDependers) == 0 {
		log.Debugf("node creation graph is successfully validated as being acyclic")

		return true
	}

	// debug output
	var d []string
	for dependee, dependers := range nodeDependers {
		d = append(d, fmt.Sprintf("%s <- [ %s ]", dependee, strings.Join(dependers, ", ")))
	}
	log.Debugf("- cycle check round %d - \n%s", i, strings.Join(d, "\n"))

	remainingNodeDependers := map[string][]string{}
	var leafNodes []string
	// mark a node as a remaining dependency if other nodes still depend on it,
	// otherwise add it to the leaf list for it to be removed in the next round of recursive check
	for dependee, dependers := range nodeDependers {
		if len(dependers) > 0 {
			remainingNodeDependers[dependee] = dependers
		} else {
			leafNodes = append(leafNodes, dependee)
		}
	}

	// if nodes remain but none of them is a leaf node, must by cyclic
	if len(leafNodes) == 0 {
		return false
	}

	// iterate over remaining nodes, to remove all leaf nodes from the dependencies, because in the next round of recursion,
	// these will no longer be there, they suffice the satisfy the acyclicity property
	for dependee, dependers := range remainingNodeDependers {
		// new array that keeps track of remaining dependencies
		var newRemainingNodeDependers []string
		// iterate over deleted nodes
		for _, dep := range dependers {
			keep := true
			// check if the actual dep is a leafNode and should therefore be removed
			for _, delnode := range leafNodes {
				// if it is a node that is meant to be deleted, stop here and make sure its not taken over to the new array
				if delnode == dep {
					keep = false
					break
				}
			}
			if keep {
				newRemainingNodeDependers = append(newRemainingNodeDependers, dep)
			}
		}
		// replace previous with the new, cleanup dependencies.
		remainingNodeDependers[dependee] = newRemainingNodeDependers
	}
	return isAcyclic(remainingNodeDependers, i+1)
}

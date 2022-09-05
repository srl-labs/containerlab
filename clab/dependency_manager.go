package clab

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type dependencyManager interface {
	// AddNode adds a node to the dependency manager.
	AddNode(name string)
	// AddDependency adds a dependency between depender and dependee.
	// The depender will effectively wait for the dependee to finish.
	AddDependency(dependee, depender string)
	// WaitForNodeDependencies is called by a node that is meant to be created.
	// This call will bock until all the nodes that this node depends on are created.
	WaitForNodeDependencies(nodeName string)
	// SignalDone is called by a node that has finished the creation process.
	// internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
	SignalDone(nodeName string)
	// CheckAcyclicity checks if dependencies contain cycles.
	CheckAcyclicity() error
	// String returns a string representation of dependencies recorded with dependency manager.
	String() string
}

type dependencyManagerImpl struct {
	// map of wait group per node.
	// The scheduling of the nodes creation is dependent on their respective wait group.
	// Other nodes, that the specific node relies on will increment this wait group.
	nodeWaitGroup map[string]*sync.WaitGroup
	// Names of the nodes that depend on a given node are listed here.
	// On successful creation of the said node, all the depending nodes (dependers) wait groups will be decremented.
	nodeDependers map[string][]string
}

func NewDependencyManager() dependencyManager {
	return &dependencyManagerImpl{
		nodeWaitGroup: map[string]*sync.WaitGroup{},
		nodeDependers: map[string][]string{},
	}
}

// AddNode adds a node to the dependency manager.
func (dm *dependencyManagerImpl) AddNode(name string) {
	dm.nodeWaitGroup[name] = &sync.WaitGroup{}
	dm.nodeDependers[name] = []string{}
}

// AddDependency adds a dependency between depender and dependee.
// The depender will effectively wait for the dependee to finish.
func (dm *dependencyManagerImpl) AddDependency(dependee, depender string) {
	dm.nodeWaitGroup[depender].Add(1)
	// add a depender node name for a given dependee
	dm.nodeDependers[dependee] = append(dm.nodeDependers[dependee], depender)
}

// WaitForNodeDependencies is called by a node that is meant to be created.
// This call will bock until all the nodes that this node depends on are created.
func (dm *dependencyManagerImpl) WaitForNodeDependencies(nodeName string) {
	dm.nodeWaitGroup[nodeName].Wait()
}

// SignalDone is called by a node that has finished the creation process.
// internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
func (dm *dependencyManagerImpl) SignalDone(nodeName string) {
	for _, depender := range dm.nodeDependers[nodeName] {
		dm.nodeWaitGroup[depender].Done()
	}
}

// CheckAcyclicity checks if dependencies contain cycles.
func (dm *dependencyManagerImpl) CheckAcyclicity() error {
	log.Debugf("Dependencies:\n%s", dm.String())
	if !isAcyclic(dm.nodeDependers, 1) {
		return fmt.Errorf("cyclic dependencies found!\n%s", dm.String())
	}

	return nil
}

// String returns a string representation of dependencies recorded with dependency manager.
func (dm *dependencyManagerImpl) String() string {
	// since dm.nodeDependers contains a map of dependee->[dependers] it is not
	// particularly suitable for displaying the dependency graph
	// this function reverses the order so that it becomes depender->[dependees]

	// map to record the dependencies in string based representation
	dependencies := map[string][]string{}

	// prepare dependencies table
	for name := range dm.nodeWaitGroup {
		dependencies[name] = []string{}
	}

	// build the dependency datastruct
	for dependee, dependers := range dm.nodeDependers {
		for _, depender := range dependers {
			dependencies[depender] = append(dependencies[depender], dependee)
		}
	}

	result := []string{}
	// print dependencies
	for nodename, deps := range dependencies {
		result = append(result, fmt.Sprintf("%s -> [ %s ]", nodename, strings.Join(deps, ", ")))
	}
	return strings.Join(result, "\n")
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
	d := []string{}
	for dependee, dependers := range nodeDependers {
		d = append(d, fmt.Sprintf("%s <- [ %s ]", dependee, strings.Join(dependers, ", ")))
	}
	log.Debugf("- cycle check round %d - \n%s", i, strings.Join(d, "\n"))

	remainingNodeDependers := map[string][]string{}
	leafNodes := []string{}
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
		newRemainingNodeDependers := []string{}
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

package dependency_manager

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/types"
)

type DependencyManager interface {
	// AddNode adds a node to the dependency manager.
	AddNode(name string)
	// AddDependency adds a dependency between depender and dependee.
	// The depender will effectively wait for the dependee to finish.
	AddDependency(depender, dependee string, state types.WaitForPhase) error
	// WaitForNodeDependencies is called by a node that is meant to be created.
	// This call will bock until all the nodes that this node depends on are created.
	Enter(nodeName string, state types.WaitForPhase) error
	// SignalDone is called by a node that has finished all tasks for the provided State.
	// internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
	SignalDone(nodeName string, state types.WaitForPhase)
	// CheckAcyclicity checks if dependencies contain cycles.
	CheckAcyclicity() error
	// String returns a string representation of dependencies recorded with dependency manager.
	String() string
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
func (dm *defaultDependencyManager) AddDependency(depender, dependee string, state types.WaitForPhase) error {
	// first check if the referenced nodes are known to the dm
	err := dm.checkNodesExist([]string{dependee})
	if err != nil {
		return err
	}
	err = dm.checkNodesExist([]string{depender})
	if err != nil {
		return err
	}

	dm.nodes[dependee].addDepender(dm.nodes[depender], state)
	return nil
}

func (dm *defaultDependencyManager) Enter(nodeName string, state types.WaitForPhase) error {
	// first check if the referenced node is known to the dm
	err := dm.checkNodesExist([]string{nodeName})
	if err != nil {
		return err
	}
	dm.nodes[nodeName].Enter(state)
	return nil
}

// SignalDone is called by a node that has finished the indicated NodeState process and reached the desired state.
// Internally the dependent nodes will be "notified" that an additional (if multiple exist) dependency is satisfied.
func (dm *defaultDependencyManager) SignalDone(nodeName string, state types.WaitForPhase) {
	// first check if the referenced node is known to the dm
	err := dm.checkNodesExist([]string{nodeName})
	if err != nil {
		log.Error(err)
		return
	}
	dm.nodes[nodeName].Done(state)
}

// checkNodesExist returns an error if any of the provided node names is unknown to the depenedency manager
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

// String returns a string representation of dependencies recorded with dependency manager.
func (dm *defaultDependencyManager) String() string {
	dependencies := dm.generateDependencyMap()

	var result []string
	// print dependencies
	for nodename, deps := range dependencies {
		result = append(result, fmt.Sprintf("%s -> [ %s ]", nodename, strings.Join(deps, ", ")))
	}
	return strings.Join(result, "\n")
}

func (dm *defaultDependencyManager) generateDependencyMap() map[string][]string {
	// map to record the dependencies in string based representation
	dependencies := map[string][]string{}

	// build the dependency datastruct
	for nodeName, node := range dm.nodes {
		dependencies[nodeName] = []string{}
		for _, perStateDependerNSSlice := range node.depender {
			for _, dependerNS := range perStateDependerNSSlice {
				dependencies[nodeName] = append(dependencies[nodeName], dependerNS.depender.name)
			}
		}
	}
	return dependencies
}

// CheckAcyclicity checks if dependencies contain cycles.
func (dm *defaultDependencyManager) CheckAcyclicity() error {
	log.Debugf("Dependencies:\n%s", dm.String())

	nodeDependers := dm.generateDependencyMap()

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

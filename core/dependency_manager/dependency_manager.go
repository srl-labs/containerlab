package dependency_manager

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	clabnodes "github.com/srl-labs/containerlab/nodes"
)

type DependencyManager interface {
	// AddNode adds a node to the dependency manager.
	AddNode(node clabnodes.Node)
	// GetNode gets a dependency node registered with the dependency manager.
	GetNode(name string) (*DependencyNode, error)
	// CheckAcyclicity checks if dependencies contain cycles.
	CheckAcyclicity() error
	// String returns a string representation of dependencies recorded with dependency manager.
	String() string
	// GetNodes returns the DependencyNodes registered with the DependencyManager
	GetNodes() map[string]*DependencyNode
}

// defaultDependencyManager is the default implementation of the DependencyManager.
type defaultDependencyManager struct {
	nodes map[string]*DependencyNode
}

// NewDependencyManager constructor.
func NewDependencyManager() DependencyManager {
	return &defaultDependencyManager{
		nodes: map[string]*DependencyNode{},
	}
}

// AddNode adds a node to the dependency manager.
func (dm *defaultDependencyManager) AddNode(node clabnodes.Node) {
	dm.nodes[node.GetShortName()] = NewDependencyNode(node)
}

// GetNodes returns the DependencyNodes registered with the DependencyManager.
func (dm *defaultDependencyManager) GetNodes() map[string]*DependencyNode {
	return dm.nodes
}

func (dm *defaultDependencyManager) GetNode(nodeName string) (*DependencyNode, error) {
	// first check if the referenced node is known to the dm
	err := dm.checkNodesExist([]string{nodeName})
	if err != nil {
		return nil, err
	}

	return dm.nodes[nodeName], err
}

// checkNodesExist returns an error if any of the provided node names is unknown to the
// depenedency manager.
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

	return fmt.Errorf(
		"dependency manager has no notion of the following nodes %s", strings.Join(missing, ", "),
	)
}

// String returns a string representation of dependencies recorded with dependency manager.
func (dm *defaultDependencyManager) String() string {
	dependencies := dm.generateDependencyMap()

	keys := make([]string, 0, len(dependencies))
	for k := range dependencies {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	result := make([]string, len(keys))

	for idx, nodename := range keys {
		result[idx] = fmt.Sprintf(
			"%s -> [ %s ]",
			nodename,
			strings.Join(dependencies[nodename], ", "),
		)
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
				dependencies[nodeName] = append(
					dependencies[nodeName],
					dependerNS.depender.GetShortName(),
				)
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
	d := make([]string, 0, len(nodeDependers))

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

	// iterate over remaining nodes, to remove all leaf nodes from the dependencies, because
	// in the next round of recursion, these will no longer be there, they suffice the satisfy
	// the acyclicity property
	for dependee, dependers := range remainingNodeDependers {
		// new array that keeps track of remaining dependencies
		var newRemainingNodeDependers []string
		// iterate over deleted nodes
		for _, dep := range dependers {
			keep := true
			// check if the actual dep is a leafNode and should therefore be removed
			for _, delnode := range leafNodes {
				// if it is a node that is meant to be deleted, stop here and make sure its not
				// taken over to the new array
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

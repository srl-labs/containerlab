package nodes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/srl-labs/containerlab/types"
)

type Initializer func() Node

type NodeRegistry struct {
	// the nodeindex is a helping struct to speedup kind lookups.
	nodeIndex map[string]*NodeRegistryEntry
}

// NewNodeRegistry constructs a new Registry.
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodeIndex: map[string]*NodeRegistryEntry{},
	}
}

// Register registers the node' init function for all provided names.
func (r *NodeRegistry) Register(names []string, initf Initializer, credentials *Credentials, kindproperties *types.KindProperties) error {
	newEntry := newRegistryEntry(names, initf, credentials, kindproperties)
	return r.addEntry(newEntry)
}

// addEntry adds the node entry to the registry.
func (r *NodeRegistry) addEntry(entry *NodeRegistryEntry) error {
	for _, name := range entry.nodeKindNames {
		if _, exists := r.nodeIndex[name]; exists {
			return fmt.Errorf("node kind %q already registered in Node Registry", name)
		}

		r.nodeIndex[name] = entry
	}

	return nil
}

// NewNodeOfKind return a new Node of the given Node Kind.
func (r *NodeRegistry) NewNodeOfKind(nodeKindName string) (Node, error) {
	nodeKindEntry, ok := r.nodeIndex[nodeKindName]
	if !ok {
		registeredKinds := strings.Join(r.GetRegisteredNodeKindNames(), ", ")
		return nil, fmt.Errorf("kind %q is not supported. Supported kinds are %q", nodeKindName, registeredKinds)
	}

	// return a new instance of the requested node
	return nodeKindEntry.initFunction(), nil
}

// GetRegisteredNodeKindNames returns a sorted slice of all the registered node kind names in the registry.
func (r *NodeRegistry) GetRegisteredNodeKindNames() []string {
	var result []string
	for k := range r.nodeIndex {
		result = append(result, k)
	}
	// sort and return
	sort.Strings(result)

	return result
}

// GetKindEntry returns a sorted slice of all the registered node kind names in the registry.
func (r *NodeRegistry) GetKindEntry(name string) *NodeRegistryEntry {
	return r.nodeIndex[name]
}

type NodeRegistryEntry struct {
	nodeKindNames []string
	initFunction  Initializer
	credentials   *Credentials
	// containes properties that are kind specific
	kindProperties *types.KindProperties
}

// GetKindProperties retrieve the kind properties
func (n *NodeRegistryEntry) GetKindProperties() *types.KindProperties {
	return n.kindProperties
}

func newRegistryEntry(nodeKindNames []string, initFunction Initializer, credentials *Credentials, kindProperties *types.KindProperties) *NodeRegistryEntry {
	// default KindProperties if nil
	if kindProperties == nil {
		kindProperties = types.NewKindProperties()
	}
	return &NodeRegistryEntry{
		nodeKindNames:  nodeKindNames,
		initFunction:   initFunction,
		credentials:    credentials,
		kindProperties: kindProperties,
	}
}

// Credentials defines NOS SSH credentials.
type Credentials struct {
	username string
	password string
}

// NewCredentials constructor for the Credentials struct.
func NewCredentials(username, password string) *Credentials {
	return &Credentials{
		username: username,
		password: password,
	}
}

func (c *Credentials) GetUsername() string {
	return c.username
}

func (c *Credentials) GetPassword() string {
	return c.password
}

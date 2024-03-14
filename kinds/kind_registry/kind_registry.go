package kind_registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/srl-labs/containerlab/nodes"
)

var KindRegistryInstance = newKindRegistry()

type Initializer func() nodes.Node

type KindRegistry struct {
	// the nodeindex is a helping struct to speedup kind lookups.
	nodeIndex map[string]*nodeRegistryEntry
}

// newKindRegistry constructs a new Registry.
func newKindRegistry() *KindRegistry {
	r := &KindRegistry{
		nodeIndex: map[string]*nodeRegistryEntry{},
	}
	return r
}

// Register registers the node' init function for all provided names.
func (r *KindRegistry) Register(names []string, initf Initializer, credentials *Credentials) error {
	newEntry := newRegistryEntry(names, initf, credentials)
	return r.addEntry(newEntry)
}

// addEntry adds the node entry to the registry.
func (r *KindRegistry) addEntry(entry *nodeRegistryEntry) error {
	for _, name := range entry.nodeKindNames {
		if _, exists := r.nodeIndex[name]; exists {
			return fmt.Errorf("node kind %q already registered in Node Registry", name)
		}

		r.nodeIndex[name] = entry
	}

	return nil
}

// NewNodeOfKind return a new Node of the given Node Kind.
func (r *KindRegistry) NewNodeOfKind(nodeKindName string) (nodes.Node, error) {
	nodeKindEntry, ok := r.nodeIndex[nodeKindName]
	if !ok {
		registeredKinds := strings.Join(r.GetRegisteredNodeKindNames(), ", ")
		return nil, fmt.Errorf("kind %q is not supported. Supported kinds are %q", nodeKindName, registeredKinds)
	}

	// return a new instance of the requested node
	return nodeKindEntry.initFunction(), nil
}

// GetRegisteredNodeKindNames returns a sorted slice of all the registered node kind names in the registry.
func (r *KindRegistry) GetRegisteredNodeKindNames() []string {
	var result []string
	for k := range r.nodeIndex {
		result = append(result, k)
	}
	// sort and return
	sort.Strings(result)

	return result
}

func (r *KindRegistry) Kind(kind string) *nodeRegistryEntry {
	return r.nodeIndex[kind]
}

type nodeRegistryEntry struct {
	nodeKindNames []string
	initFunction  Initializer
	credentials   *Credentials
}

// Credentials returns entry's credentials.
func (e *nodeRegistryEntry) Credentials() *Credentials {
	if e == nil {
		return nil
	}

	return e.credentials
}

func newRegistryEntry(nodeKindNames []string, initFunction Initializer, credentials *Credentials) *nodeRegistryEntry {
	return &nodeRegistryEntry{
		nodeKindNames: nodeKindNames,
		initFunction:  initFunction,
		credentials:   credentials,
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
	if c == nil {
		return ""
	}

	return c.username
}

func (c *Credentials) GetPassword() string {
	if c == nil {
		return ""
	}

	return c.password
}

// Slice returns credentials as a slice.
func (c *Credentials) Slice() []string {
	if c == nil {
		return nil
	}

	return []string{c.GetUsername(), c.GetPassword()}
}

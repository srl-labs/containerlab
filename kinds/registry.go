package kinds

import (
	"fmt"
	"sort"
	"strings"

	"github.com/srl-labs/containerlab/nodes"
)

type Initializer func() nodes.Node

type Registry struct {
	kinds map[string]*kind
}

type kind struct {
	newNode     Initializer
	credentials *Credentials
}

// NewRegistry constructs a new Registry.
func NewRegistry() *Registry {
	return &Registry{
		kinds: map[string]*kind{},
	}
}

// AddKnownKind registers the node's init function for all provided names.
func (r *Registry) AddKnownKind(names []string, newNode Initializer, credentials *Credentials) error {
	k := &kind{
		newNode:     newNode,
		credentials: credentials,
	}

	for _, name := range names {
		if _, exists := r.kinds[name]; exists {
			return fmt.Errorf("node kind %q already registered in Node Registry", name)
		}
		r.kinds[name] = k
	}

	return nil
}

// NewNodeOfKind return a new Node of the given Node Kind.
func (r *Registry) NewNodeOfKind(name string) (nodes.Node, error) {
	nodeKindEntry, ok := r.kinds[name]
	if !ok {
		registeredKinds := strings.Join(r.getRegisteredKindNames(), ", ")
		return nil, fmt.Errorf("kind %q is not supported. Supported kinds are %q", name, registeredKinds)
	}
	// return a new instance of the requested node
	return nodeKindEntry.newNode(), nil
}

// getRegisteredKindNames returns a sorted slice of all the registered node kind names in the registry.
func (r *Registry) getRegisteredKindNames() []string {
	var result []string
	for k := range r.kinds {
		result = append(result, k)
	}
	// sort and return
	sort.Strings(result)

	return result
}

func (r *Registry) GetCredentialsOfKind(name string) *Credentials {
	k, ok := r.kinds[name]
	if !ok {
		return nil
	}
	return k.credentials
}

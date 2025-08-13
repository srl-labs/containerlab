package nodes

import (
	"fmt"
	"sort"
	"strings"
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
func (r *NodeRegistry) Register(names []string, initf Initializer, attributes *NodeRegistryEntryAttributes) error {
	newEntry := newRegistryEntry(names, initf, attributes)
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

func (r *NodeRegistry) GetGenerateNodeAttributes() map[string]*GenerateNodeAttributes {
	result := map[string]*GenerateNodeAttributes{}
	for k, v := range r.nodeIndex {
		result[k] = v.GetGenerateAttributes()
	}
	return result
}

func (r *NodeRegistry) Kind(kind string) *NodeRegistryEntry {
	return r.nodeIndex[kind]
}

type NodeRegistryEntry struct {
	nodeKindNames []string
	initFunction  Initializer
	attributes    *NodeRegistryEntryAttributes
}

func (nre *NodeRegistryEntry) GetCredentials() *Credentials {
	if nre.attributes == nil {
		return nil
	}

	return nre.attributes.GetCredentials()
}

type NodeRegistryEntryAttributes struct {
	credentials        *Credentials
	generateAttributes *GenerateNodeAttributes
	platformAttrs      *PlatformAttrs
}

func (nre *NodeRegistryEntry) GetGenerateAttributes() *GenerateNodeAttributes {
	if nre.attributes == nil {
		return nil
	}
	return nre.attributes.generateAttributes
}

func (nre *NodeRegistryEntry) PlatformAttrs() *PlatformAttrs {
	if nre.attributes == nil {
		return nil
	}

	return nre.attributes.PlatformAttrs()
}

// Credentials returns entry's credentials.
// might return nil if no default credentials present.
func (e *NodeRegistryEntryAttributes) Credentials() *Credentials {
	if e == nil {
		return nil
	}

	return e.credentials
}

func newRegistryEntry(nodeKindNames []string, initFunction Initializer,
	attributes *NodeRegistryEntryAttributes,
) *NodeRegistryEntry {
	return &NodeRegistryEntry{
		nodeKindNames: nodeKindNames,
		initFunction:  initFunction,
		attributes:    attributes,
	}
}

// PlatformAttrs contains the platform attributes this node/platform is known to have in different libraries and tools.
// Most often just the platform/provider name.
type PlatformAttrs struct {
	ScrapliPlatformName string
	NapalmPlatformName  string
}

// NewNodeRegistryEntryAttributes creates a new NodeRegistryEntryAttributes.
func NewNodeRegistryEntryAttributes(c *Credentials, ga *GenerateNodeAttributes, pa *PlatformAttrs) *NodeRegistryEntryAttributes {
	// set default value for GenerateNodeAttributes
	if ga == nil {
		ga = NewGenerateNodeAttributes(false, "")
	}
	return &NodeRegistryEntryAttributes{
		credentials:        c,
		generateAttributes: ga,
		platformAttrs:      pa,
	}
}

func (nrea *NodeRegistryEntryAttributes) GetCredentials() *Credentials {
	return nrea.credentials
}

func (nrea *NodeRegistryEntryAttributes) GetGenerateAttributes() *GenerateNodeAttributes {
	return nrea.generateAttributes
}

// PlatformAttrs returns the platform attributes of this node's registry attributes.
func (nrea *NodeRegistryEntryAttributes) PlatformAttrs() *PlatformAttrs {
	if nrea == nil {
		return nil
	}

	return nrea.platformAttrs
}

type GenerateNodeAttributes struct {
	generateable    bool
	interfaceFormat string
}

func NewGenerateNodeAttributes(generateable bool, ifFormat string) *GenerateNodeAttributes {
	return &GenerateNodeAttributes{
		generateable:    generateable,
		interfaceFormat: ifFormat,
	}
}

func (ga *GenerateNodeAttributes) IsGenerateable() bool {
	if ga == nil {
		return false
	}
	return ga.generateable
}

func (ga *GenerateNodeAttributes) GetInterfaceFormat() string {
	if ga == nil {
		return ""
	}
	return ga.interfaceFormat
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

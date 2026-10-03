package core

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// decodeKindConfig decodes the node's raw kind config keys from topo into its kind's config type.
func (c *CLab) decodeKindConfig(topo *clabtypes.Topology, nodeName, kind string) (any, error) {
	var entry *clabnodes.NodeRegistryEntry
	if c.Reg != nil {
		entry = c.Reg.Kind(kind)
	}

	return clabnodes.DecodeKindConfig(entry, nodeName, kind, topo.GetNodeKindConfig(nodeName))
}

// validateKindConfigKeys checks the kind config keys of the defaults, kinds and groups blocks,
// including blocks no node uses: keys under kinds.<kind> must belong to that kind, keys of a
// defaults or group block that sets a kind must belong to that kind, and every other key must
// belong to at least one kind. Node blocks are checked when each node's kind config is decoded.
func (c *CLab) validateKindConfigKeys() error {
	if c.Reg == nil {
		return nil
	}

	topo := c.Config.Topology

	var errs []error

	check := func(from, kind string, def *clabtypes.NodeDefinition) {
		if def == nil {
			return
		}

		for _, key := range slices.Sorted(maps.Keys(def.KindConfig)) {
			switch {
			case kind != "":
				if !c.Reg.Kind(kind).AcceptsKindConfigKey(key) {
					errs = append(errs, fmt.Errorf("kind %q does not support key %q (set in %s)",
						kind, key, from))
				}
			case !c.anyKindAcceptsKindConfigKey(key):
				errs = append(errs, fmt.Errorf("no kind supports key %q (set in %s)", key, from))
			}
		}
	}

	check("defaults", strings.ToLower(topo.GetDefaults().Kind), topo.GetDefaults())

	for _, name := range slices.Sorted(maps.Keys(topo.Kinds)) {
		check("kinds."+name, strings.ToLower(name), topo.Kinds[name])
	}

	for _, name := range slices.Sorted(maps.Keys(topo.Groups)) {
		if def := topo.Groups[name]; def != nil {
			check("groups."+name, strings.ToLower(def.Kind), def)
		}
	}

	return errors.Join(errs...)
}

func (c *CLab) anyKindAcceptsKindConfigKey(key string) bool {
	for _, kind := range c.Reg.GetRegisteredNodeKindNames() {
		if c.Reg.Kind(kind).AcceptsKindConfigKey(key) {
			return true
		}
	}

	return false
}

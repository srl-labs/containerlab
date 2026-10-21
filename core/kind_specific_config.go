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

// decodeKindSpecificConfig decodes the node's raw kind-specific config keys from topo into its
// kind's config type.
func (c *CLab) decodeKindSpecificConfig(
	topo *clabtypes.Topology,
	nodeName, kind string,
) (any, error) {
	var entry *clabnodes.NodeRegistryEntry
	if c.Reg != nil {
		entry = c.Reg.Kind(kind)
	}

	return clabnodes.DecodeKindSpecificConfig(
		entry,
		nodeName,
		kind,
		c.expandKindSpecificConfigMagicVars(
			topo.GetNodeKindSpecificConfig(nodeName),
			nodeName,
		),
	)
}

// expandKindSpecificConfigMagicVars replaces magic variables in every string of the given raw
// kind-specific config entries. Values are copied, so the topology definitions the entries point
// at are left untouched and keep serving other nodes unchanged.
func (c *CLab) expandKindSpecificConfigMagicVars(
	entries []clabtypes.KindSpecificConfigEntry,
	nodeName string,
) []clabtypes.KindSpecificConfigEntry {
	if c.TopoPaths == nil || !c.TopoPaths.TopologyFileIsSet() {
		// Without the topology paths the magic variables cannot be resolved; the entries are
		// returned as parsed.
		return entries
	}

	r := c.magicVarReplacer(nodeName)

	var expand func(any) any

	expand = func(v any) any {
		switch v := v.(type) {
		case string:
			return r.Replace(v)
		case []any:
			out := make([]any, len(v))
			for i, e := range v {
				out[i] = expand(e)
			}

			return out
		case map[string]any:
			out := make(map[string]any, len(v))
			for k, e := range v {
				out[k] = expand(e)
			}

			return out
		case map[any]any:
			out := make(map[any]any, len(v))
			for k, e := range v {
				out[expand(k)] = expand(e)
			}

			return out
		default:
			return v
		}
	}

	out := make([]clabtypes.KindSpecificConfigEntry, len(entries))
	for i, e := range entries {
		out[i] = e
		out[i].Value = expand(e.Value)
	}

	return out
}

// validateKindSpecificConfigKeys checks the kind-specific config keys of the defaults, kinds and
// groups blocks,
// including blocks no node uses: keys under kinds.<kind> must belong to that kind, keys of a
// defaults or group block that sets a kind must belong to that kind, and every other key must
// belong to at least one kind. Node blocks are checked when each node's kind-specific config is
// decoded.
func (c *CLab) validateKindSpecificConfigKeys() error {
	if c.Reg == nil {
		return nil
	}

	topo := c.Config.Topology

	var errs []error

	check := func(from, kind string, def *clabtypes.NodeDefinition) {
		if def == nil {
			return
		}

		for _, key := range slices.Sorted(maps.Keys(def.KindSpecificConfig)) {
			switch {
			case kind != "":
				if !c.Reg.Kind(kind).AcceptsKindSpecificConfigKey(key) {
					errs = append(errs, fmt.Errorf("kind %q does not support key %q (set in %s)",
						kind, key, from))
				}
			case !c.anyKindAcceptsKindSpecificConfigKey(key):
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

func (c *CLab) anyKindAcceptsKindSpecificConfigKey(key string) bool {
	for _, kind := range c.Reg.GetRegisteredNodeKindNames() {
		if c.Reg.Kind(kind).AcceptsKindSpecificConfigKey(key) {
			return true
		}
	}

	return false
}

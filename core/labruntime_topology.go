package core

import (
	"fmt"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
	"gopkg.in/yaml.v2"
)

// labRuntimeTopologyWithExpandedMagicVars renders the per-node values that local runtimes
// normally resolve while initializing nodes. Inherited fields are materialized on each node so
// that defaults, kinds, and groups can contain node-scoped variables too.
func (c *CLab) labRuntimeTopologyWithExpandedMagicVars() ([]byte, error) {
	if !strings.Contains(string(c.renderedTopology), "__clab") {
		return c.renderedTopology, nil
	}

	source := &Config{}
	if err := yaml.Unmarshal(c.renderedTopology, source); err != nil {
		return nil, fmt.Errorf(
			"failed to parse rendered topology for magic variable expansion: %w",
			err,
		)
	}
	if source.Topology == nil {
		return c.renderedTopology, nil
	}

	expanded := &Config{}
	if err := yaml.Unmarshal(c.renderedTopology, expanded); err != nil {
		return nil, fmt.Errorf(
			"failed to copy rendered topology for magic variable expansion: %w",
			err,
		)
	}

	for nodeName := range source.Topology.Nodes {
		if err := c.expandLabRuntimeNodeMagicVars(
			source.Topology,
			expanded.Topology,
			nodeName,
		); err != nil {
			return nil, err
		}
	}
	clearInheritedLabRuntimeNodeFields(expanded.Topology)

	var document yaml.MapSlice
	if err := yaml.Unmarshal(c.renderedTopology, &document); err != nil {
		return nil, fmt.Errorf("failed to parse rendered topology document: %w", err)
	}
	for i := range document {
		if document[i].Key == "topology" {
			document[i].Value = expanded.Topology
			break
		}
	}

	result, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("failed to render topology with expanded magic variables: %w", err)
	}

	return result, nil
}

func (c *CLab) expandLabRuntimeNodeMagicVars(
	source, expanded *clabtypes.Topology,
	nodeName string,
) error {
	node := expanded.Nodes[nodeName]
	if node == nil {
		node = &clabtypes.NodeDefinition{}
		expanded.Nodes[nodeName] = node
	}

	r := c.magicVarReplacer(nodeName)
	node.StartupConfig = r.Replace(source.GetNodeStartupConfig(nodeName))

	var err error
	node.Binds, err = source.GetNodeBinds(nodeName)
	if err != nil {
		return fmt.Errorf(
			"failed to resolve binds for node %q during magic variable expansion: %w",
			nodeName,
			err,
		)
	}
	for i := range node.Binds {
		parts := strings.Split(node.Binds[i], ":")
		parts[0] = r.Replace(parts[0])
		node.Binds[i] = strings.Join(parts, ":")
	}

	node.Volumes, err = source.GetNodeVolumes(nodeName)
	if err != nil {
		return fmt.Errorf(
			"failed to resolve volumes for node %q during magic variable expansion: %w",
			nodeName,
			err,
		)
	}
	for i := range node.Volumes {
		parts := strings.Split(node.Volumes[i], ":")
		if len(parts) == 1 {
			continue
		}
		parts[0] = r.Replace(parts[0])
		node.Volumes[i] = strings.Join(parts, ":")
	}

	node.Exec = source.GetNodeExec(nodeName)
	if labRuntimeNodeHasStages(source, nodeName) {
		node.Stages, err = source.GetStages(nodeName)
		if err != nil {
			return fmt.Errorf(
				"failed to resolve stages for node %q during magic variable expansion: %w",
				nodeName,
				err,
			)
		}
	}
	nodeConfig := &clabtypes.NodeConfig{
		ShortName: nodeName,
		Exec:      node.Exec,
		Stages:    node.Stages,
	}
	c.processNodeExecs(nodeConfig)
	node.Exec = nodeConfig.Exec
	node.Stages = nodeConfig.Stages

	entries := c.expandKindSpecificConfigMagicVars(
		source.GetNodeKindSpecificConfig(nodeName), nodeName,
	)
	if len(entries) != 0 {
		node.KindSpecificConfig = make(map[string]any, len(entries))
		for _, entry := range entries {
			node.KindSpecificConfig[entry.Key] = entry.Value
		}
	}

	return nil
}

func labRuntimeNodeHasStages(topology *clabtypes.Topology, nodeName string) bool {
	definitions := []*clabtypes.NodeDefinition{
		topology.Defaults,
		topology.GetKind(topology.GetNodeKind(nodeName)),
		topology.GetGroup(topology.GetNodeGroup(nodeName)),
		topology.Nodes[nodeName],
	}
	for _, definition := range definitions {
		if definition != nil && definition.Stages != nil {
			return true
		}
	}

	return false
}

// clearInheritedLabRuntimeNodeFields prevents the inherited values from being merged a second
// time after their effective, expanded copies have been placed on every node.
func clearInheritedLabRuntimeNodeFields(topology *clabtypes.Topology) {
	definitions := []*clabtypes.NodeDefinition{topology.Defaults}
	for _, definition := range topology.Kinds {
		definitions = append(definitions, definition)
	}
	for _, definition := range topology.Groups {
		definitions = append(definitions, definition)
	}

	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		definition.StartupConfig = ""
		definition.Binds = nil
		definition.Volumes = nil
		definition.Exec = nil
		definition.Stages = nil
		definition.KindSpecificConfig = nil
	}
}

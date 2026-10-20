package clabernetes

import (
	"strings"

	clabernetescompiler "github.com/clabernetes/clabernetes/compiler"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnames "github.com/srl-labs/containerlab/names"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// sanitizeNodeNames renames the topology nodes Kubernetes cannot carry, along with every reference
// to them the c9s subset understands, and reports what was renamed. c9s uses containerlab node
// names verbatim as Node, Deployment and Service names, so a node named R1 has to become r1 before
// anything is created; the alternative is rejecting a large share of public labs outright.
//
// Every node definition also carries c9s.run/originalNodeName naming the node as the topology file
// spells it, so tooling holding only the clab file can select the c9s Nodes by the names they use
// there. The label rides in the definition, which is what makes it survive both the Topology
// resource path (the controller compiles it in-cluster) and the primitives path (compiled here);
// the compiler stamps the same label on Nodes it renames itself, for topologies authored outside
// containerlab.
//
// The boolean reports whether the topology definition was modified, so the caller only re-renders
// the definition when it really changed.
func sanitizeNodeNames(
	config *clabRuntimeConfig,
	nodeNames []string,
) (map[string]string, bool, error) {
	renames, err := clabnames.SanitizeNodeNames(nodeNames)
	if err != nil {
		return nil, false, err
	}

	changed := len(renames) != 0

	nodes := make(map[string]*clabtypes.NodeDefinition, len(config.Topology.Nodes))
	for originalName, nodeDefinition := range config.Topology.Nodes {
		nodeName := originalName
		if sanitized, renamed := renames[originalName]; renamed {
			nodeName = sanitized
		}

		originalLabel := clabnames.SanitizeLabelValue(originalName)
		if nodeDefinition.Labels[labelOriginalNodeName] != originalLabel {
			if nodeDefinition.Labels == nil {
				nodeDefinition.Labels = map[string]string{}
			}
			nodeDefinition.Labels[labelOriginalNodeName] = originalLabel
			changed = true
		}

		nodes[nodeName] = nodeDefinition
	}
	config.Topology.Nodes = nodes

	if len(renames) != 0 {
		// A node can share the network namespace of another node by name, and the compiler resolves
		// that setting through the kind, group and defaults sections as well.
		for _, nodeDefinition := range referencingNodeDefinitions(config.Topology) {
			clabernetescompiler.RenameNetworkModePrimary(nodeDefinition, renames)
		}
	}

	return renames, changed, nil
}

// referencingNodeDefinitions returns every node definition of a topology that can hold a reference
// to a node name, in no particular order.
func referencingNodeDefinitions(topology *clabtypes.Topology) []*clabtypes.NodeDefinition {
	definitions := make(
		[]*clabtypes.NodeDefinition,
		0,
		len(topology.Nodes)+len(topology.Kinds)+len(topology.Groups)+1,
	)
	definitions = append(definitions, topology.Defaults)

	for _, group := range []map[string]*clabtypes.NodeDefinition{
		topology.Nodes,
		topology.Kinds,
		topology.Groups,
	} {
		for _, nodeDefinition := range group {
			definitions = append(definitions, nodeDefinition)
		}
	}

	return definitions
}

// renameBriefLinkEndpoints points the wiring at the sanitized node names. Endpoints that do not
// name a renamed node are left alone, which is what keeps the host, mgmt-net and macvlan pseudo
// endpoints intact.
func renameBriefLinkEndpoints(links []*clablinks.LinkBriefRaw, renames map[string]string) {
	if len(renames) == 0 {
		return
	}

	for _, link := range links {
		endpoints := make([]string, 0, len(link.Endpoints))

		for _, endpoint := range link.Endpoints {
			nodeName, interfaceName, hasInterface := strings.Cut(endpoint, ":")

			sanitized, renamed := renames[nodeName]
			switch {
			case !renamed:
				endpoints = append(endpoints, endpoint)
			case hasInterface:
				endpoints = append(endpoints, sanitized+":"+interfaceName)
			default:
				endpoints = append(endpoints, sanitized)
			}
		}

		link.Endpoints = endpoints
	}
}

// renameStagedConfigMapNodes points the staged file projections at the sanitized node names. The
// ConfigMaps themselves are shared by source file, but each mount and ownership reference has to
// match the Node object c9s creates.
func renameStagedConfigMapNodes(configMaps []stagedConfigMap, renames map[string]string) {
	if len(renames) == 0 {
		return
	}

	for idx := range configMaps {
		for mountIdx := range configMaps[idx].mounts {
			mount := &configMaps[idx].mounts[mountIdx]
			if sanitized, renamed := renames[mount.nodeName]; renamed {
				mount.nodeName = sanitized
			}
		}
	}
}

// resolveKnownNodeName maps a node name a user supplied onto the name the lab carries in
// Kubernetes. Node names Kubernetes cannot carry are sanitized at deploy time, so a name typed
// exactly as the topology file writes it has to keep resolving. A name that exists verbatim always
// wins: a primitive-only lab created outside containerlab may legitimately carry a name the
// sanitizer would map elsewhere.
func resolveKnownNodeName[V any](known map[string]V, nodeName string) (string, bool) {
	if _, ok := known[nodeName]; ok {
		return nodeName, true
	}

	sanitized := clabnames.SanitizeName(nodeName)
	if _, ok := known[sanitized]; ok {
		return sanitized, true
	}

	return "", false
}

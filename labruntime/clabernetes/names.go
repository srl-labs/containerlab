package clabernetes

import (
	"strings"

	clabernetesutilcontainerlab "github.com/clabernetes/clabernetes/util/containerlab"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clablinks "github.com/srl-labs/containerlab/links"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// sanitizeNodeNames renames the topology nodes Kubernetes cannot carry, along with every reference
// to them the c9s subset understands, and reports what was renamed. c9s uses containerlab node
// names verbatim as Node, Deployment and Service names, so a node named R1 has to become r1 before
// anything is created; the alternative is rejecting a large share of public labs outright.
func sanitizeNodeNames(
	config *clabRuntimeConfig,
	nodeNames []string,
) (map[string]string, error) {
	renames, err := clablabruntime.SanitizeNodeNames(nodeNames)
	if err != nil || len(renames) == 0 {
		return nil, err
	}

	nodes := make(map[string]*clabtypes.NodeDefinition, len(config.Topology.Nodes))
	for nodeName, nodeDefinition := range config.Topology.Nodes {
		if sanitized, renamed := renames[nodeName]; renamed {
			nodeName = sanitized
		}

		nodes[nodeName] = nodeDefinition
	}
	config.Topology.Nodes = nodes

	// A node can share the network namespace of another node by name, and the compiler resolves
	// that setting through the kind, group and defaults sections as well.
	for _, nodeDefinition := range referencingNodeDefinitions(config.Topology) {
		renameNetworkModePrimary(nodeDefinition, renames)
	}

	return renames, nil
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

func renameNetworkModePrimary(
	nodeDefinition *clabtypes.NodeDefinition,
	renames map[string]string,
) {
	if nodeDefinition == nil {
		return
	}

	primary := clabernetesutilcontainerlab.ParseNetworkModeContainer(nodeDefinition.NetworkMode)
	if sanitized, renamed := renames[primary]; renamed {
		nodeDefinition.NetworkMode = clabernetesutilcontainerlab.NetworkModeContainerPrefix +
			sanitized
	}
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
// ConfigMaps themselves are already named through safeKubernetesName, but the node each mount and
// ownership reference belongs to has to match the Node object c9s creates.
func renameStagedConfigMapNodes(configMaps []stagedConfigMap, renames map[string]string) {
	if len(renames) == 0 {
		return
	}

	for idx := range configMaps {
		if sanitized, renamed := renames[configMaps[idx].nodeName]; renamed {
			configMaps[idx].nodeName = sanitized
		}

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

	sanitized := clablabruntime.SanitizeName(nodeName)
	if _, ok := known[sanitized]; ok {
		return sanitized, true
	}

	return "", false
}

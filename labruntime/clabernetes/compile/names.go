// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"fmt"
	"slices"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
)

// NetworkModeContainerPrefix is the prefix of the `network-mode` node setting expressing that a
// node shares the network namespace of another (containerlab) node.
const NetworkModeContainerPrefix = "container:"

// ParseNetworkModeContainer parses a network-mode value and returns the referenced (primary)
// node name if it is a container network-mode (i.e. "container:node-a" returns "node-a"), or an
// empty string otherwise.
func ParseNetworkModeContainer(networkMode string) string {
	if !strings.HasPrefix(networkMode, NetworkModeContainerPrefix) {
		return ""
	}

	return strings.TrimPrefix(networkMode, NetworkModeContainerPrefix)
}

// RenameNetworkModePrimary points a node definition's container network-mode at the sanitized
// name of the node it shares a network namespace with. Definitions that do not reference a
// renamed node are left untouched. It serves both the compile, which renames the flattened
// nodes, and the direct runtime, which renames the topology before compiling it.
func RenameNetworkModePrimary(
	nodeDefinition *clabtypes.NodeDefinition,
	renames map[string]string,
) {
	if nodeDefinition == nil {
		return
	}

	primary := ParseNetworkModeContainer(nodeDefinition.NetworkMode)

	compiledName, renamed := renames[primary]
	if !renamed {
		return
	}

	nodeDefinition.NetworkMode = NetworkModeContainerPrefix + compiledName
}

// sanitizeCompiledNodeNames renames the compiled nodes whose containerlab name Kubernetes cannot
// carry, and points every reference to them at the new name. c9s names the Node, Deployment and
// Service objects after the containerlab node, so a topology naming its routers R1..R5 -- the most
// common convention in public labs -- would otherwise be rejected by the API server with nothing
// but a reconcile loop to show for it.
//
// The source name of every renamed node is kept on the compiled topology so the renderers can
// still find the node-keyed policy the Topology author wrote.
func sanitizeCompiledNodeNames(
	logger Logger,
	compiled *CompiledTopology,
	diagnostics *compileDiagnostics,
) {
	renames, err := nodeNameRenames(compiled.Nodes)
	if err != nil {
		diagnostics.add(Diagnostic{
			Code:    "colliding-node-names",
			Path:    "topology.nodes",
			Message: err.Error(),
		})

		return
	}

	if len(renames) == 0 {
		return
	}

	nodes := make(map[string]*clabtypes.NodeDefinition, len(compiled.Nodes))
	sources := make(map[string]string, len(renames))

	for nodeName, nodeDefinition := range compiled.Nodes {
		compiledName, renamed := renames[nodeName]
		if renamed {
			sources[compiledName] = nodeName
		} else {
			compiledName = nodeName
		}

		nodes[compiledName] = nodeDefinition
	}

	compiled.Nodes = nodes
	compiled.NodeNameSources = sources
	if len(compiled.AppProtocols) != 0 {
		appProtocols := make(
			map[string][]AppProtocol,
			len(compiled.AppProtocols),
		)

		for nodeName, entries := range compiled.AppProtocols {
			compiledName, renamed := renames[nodeName]
			if !renamed {
				compiledName = nodeName
			}

			appProtocols[compiledName] = entries
		}

		compiled.AppProtocols = appProtocols
	}

	for _, nodeDefinition := range compiled.Nodes {
		RenameNetworkModePrimary(nodeDefinition, renames)
	}

	for idx := range compiled.Links {
		renameLinkEndpointNode(&compiled.Links[idx].EndpointA, renames)
		renameLinkEndpointNode(&compiled.Links[idx].EndpointB, renames)
	}

	logger.Warnf(
		"topology compile: node names Kubernetes cannot carry were sanitized: %s",
		formatNodeNameRenames(renames),
	)
}

// nodeNameRenames returns the compiled name of every node name that needs one, keyed by the name
// the definition uses. Two node names that differ only in something Kubernetes cannot carry (R1
// and r1) would collapse onto one object, so that is an error rather than a silent merge.
func nodeNameRenames(
	nodes map[string]*clabtypes.NodeDefinition,
) (map[string]string, error) {
	renames := map[string]string{}
	origins := make(map[string]string, len(nodes))

	for _, nodeName := range sortedNodeNames(nodes) {
		sanitized := SanitizeName(nodeName)
		if sanitized == "" {
			return nil, fmt.Errorf(
				"%w: node name %q holds no character a Kubernetes object name can be built from",
				errInvalidData,
				nodeName,
			)
		}

		if origin, taken := origins[sanitized]; taken {
			return nil, fmt.Errorf(
				"%w: node names %q and %q both map onto the Kubernetes name %q; rename one of them",
				errInvalidData,
				origin,
				nodeName,
				sanitized,
			)
		}

		origins[sanitized] = nodeName

		if sanitized != nodeName {
			renames[nodeName] = sanitized
		}
	}

	return renames, nil
}

func renameLinkEndpointNode(
	endpoint *Endpoint,
	renames map[string]string,
) {
	if compiledName, renamed := renames[endpoint.NodeName]; renamed {
		endpoint.NodeName = compiledName
	}
}

func formatNodeNameRenames(renames map[string]string) string {
	sourceNames := make([]string, 0, len(renames))
	for sourceName := range renames {
		sourceNames = append(sourceNames, sourceName)
	}

	slices.Sort(sourceNames)

	formatted := make([]string, 0, len(sourceNames))
	for _, sourceName := range sourceNames {
		formatted = append(formatted, fmt.Sprintf("%s -> %s", sourceName, renames[sourceName]))
	}

	return strings.Join(formatted, ", ")
}

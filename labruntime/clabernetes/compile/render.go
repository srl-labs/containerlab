// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	c9sAPIVersion  = "c9s.run/v1alpha1"
	nodeKind       = "Node"
	linkKind       = "Link"
	nodeProfile    = "NodeProfile"
	labelPrefix    = "c9s.run"
	labelApp       = labelPrefix + "/app"
	labelName      = labelPrefix + "/name"
	labelOwner     = labelPrefix + "/topologyOwner"
	labelKind      = labelPrefix + "/topologyKind"
	labelNode      = labelPrefix + "/topologyNode"
	labelGroup     = labelPrefix + "/topologyGroup"
	labelAppValue  = "clabernetes"
	defaultTopoKey = "default"
)

// LinkHostNodeName is the reserved endpoint node name for a node-local host link.
const LinkHostNodeName = "host"

var linkNameInvalidChars = regexp.MustCompile(`[^a-z0-9-]`)

// nameMaxLen is the maximum length for a Kubernetes object name.
const nameMaxLen = 63

// nameDigestLen is the number of digest characters appended to a name that had to be truncated
// to fit nameMaxLen.
const nameDigestLen = 7

// leadingLetterPrefix prefixes a name that would otherwise start with a digit or a dash, which
// a DNS-1035 label cannot do.
const leadingLetterPrefix = "clab-"

// nonNameChars matches every run of characters a DNS label cannot carry, once the name has been
// lower-cased.
var nonNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

// SanitizeName maps a containerlab name onto the DNS-1035 label Kubernetes can name an object
// with: lower case, made up of a-z, 0-9 and '-', starting with a letter and at most 63 characters
// long. A large share of public labs name their routers R1/PE_1, and Kubernetes cannot carry
// those names as they are.
//
// Unlike a lossy enforcement this never rewrites a character a DNS label can carry, so a name
// Kubernetes already accepts maps onto itself and the mapping is idempotent. The result is empty
// only for a name holding nothing a Kubernetes name can be built from.
func SanitizeName(name string) string {
	sanitized := strings.Trim(nonNameChars.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if sanitized == "" {
		return ""
	}

	if sanitized[0] < 'a' || sanitized[0] > 'z' {
		sanitized = leadingLetterPrefix + sanitized
	}

	if len(sanitized) > nameMaxLen {
		digest := sha256.Sum256([]byte(name))
		sanitized = strings.TrimRight(
			sanitized[:nameMaxLen-nameDigestLen-1],
			"-",
		) + "-" + hex.EncodeToString(digest[:])[:nameDigestLen]
	}

	return sanitized
}

// SafeConcatNameKubernetes concats all provided strings into a string joined by "-" - if the
// final string is greater than 63 characters, the string will be shortened, and a hash will be
// used at the end of the string to keep it unique, but safely within allowed lengths.
func SafeConcatNameKubernetes(name ...string) string {
	finalName := strings.Join(name, "-")

	if len(finalName) <= nameMaxLen {
		return finalName
	}

	digest := sha256.Sum256([]byte(finalName))

	return finalName[0:nameMaxLen-8] + "-" + hex.EncodeToString(digest[0:])[:nameDigestLen]
}

// sanitizeLinkNamePart makes an interface name safe for use inside a kubernetes object name.
// Any lossy normalization includes a hash of the raw value so distinct interfaces remain
// distinct.
func sanitizeLinkNamePart(part string) string {
	rawPart := part
	part = linkNameInvalidChars.ReplaceAllString(strings.ToLower(rawPart), "-")

	part = strings.Trim(part, "-")

	if part == "" {
		part = "x"
	}

	if part != rawPart {
		digest := sha256.Sum256([]byte(rawPart))
		part = fmt.Sprintf("%s-%x", part, digest[:4])
	}

	return part
}

// LinkResourceName returns the (deterministic) name of the Link object for the given wire.
func LinkResourceName(endpointA, endpointB Endpoint) string {
	return SafeConcatNameKubernetes(
		endpointA.NodeName,
		sanitizeLinkNamePart(endpointA.InterfaceName),
		endpointB.NodeName,
		sanitizeLinkNamePart(endpointB.InterfaceName),
	)
}

// ownedObjectMetadata returns the base metadata for objects the compiler emits: the topology's
// annotations and global labels, plus the compiler's own identifying labels.
func ownedObjectMetadata(input *Input, name string) map[string]any {
	labels := map[string]any{
		labelApp:   labelAppValue,
		labelName:  name,
		labelOwner: input.Name,
		labelKind:  TopologyKindContainerlab,
	}

	for key, value := range input.Labels {
		labels[key] = value
	}

	metadata := map[string]any{
		"name":      name,
		"labels":    labels,
		"namespace": input.Namespace,
	}

	if len(input.Annotations) != 0 {
		annotations := map[string]any{}
		for key, value := range input.Annotations {
			annotations[key] = value
		}
		metadata["annotations"] = annotations
	}

	return metadata
}

// DefinitionPayload renders the flattened node definition as the manifest payload the c9s Node
// vocabulary carries: the vocabulary's named fields at the top level, the `components`
// kind-specific key as its typed field, and every other kind-specific key wrapped under
// `kind-specific-config` so the API server preserves it. Clabernetes' compiler uses it to
// hydrate its own vocabulary from the engine's flattened definitions.
func DefinitionPayload(flattened *clabtypes.NodeDefinition) (map[string]any, error) {
	return definitionPayload(flattened)
}

// definitionPayload renders the flattened node definition as the manifest payload the c9s Node
// vocabulary carries: the vocabulary's named fields at the top level, the `components`
// kind-specific key as its typed field, and every other kind-specific key wrapped under
// `kind-specific-config` so the API server preserves it.
func definitionPayload(flattened *clabtypes.NodeDefinition) (map[string]any, error) {
	// The yaml.v2 marshal carries containerlab's yaml vocabulary and inlines the kind-specific
	// config keys via NodeDefinition.MarshalYAML; the yaml.v3 decode renders the same tree into
	// the JSON-native shapes the manifest needs.
	raw, err := yamlv2.Marshal(flattened)
	if err != nil {
		return nil, err
	}

	var payload map[string]any

	err = yamlv3.Unmarshal(raw, &payload)
	if err != nil {
		return nil, err
	}

	if payload == nil {
		payload = map[string]any{}
	}

	kindConfig := map[string]any{}

	for key := range flattened.KindSpecificConfig {
		if key == kindSpecificComponentsKey {
			// The components inventory is the one kind-specific key the c9s vocabulary carries
			// as a typed field; it stays at the top level of the payload.
			continue
		}

		// The payload already decoded the value into JSON-safe shapes.
		kindConfig[key] = payload[key]

		delete(payload, key)
	}

	if len(kindConfig) != 0 {
		payload[kindSpecificWrapperKey] = kindConfig
	}

	// containerlab carries the certificate validity as a duration; the c9s vocabulary carries
	// it as the duration string form.
	if flattened.Certificate != nil && flattened.Certificate.ValidityDuration > 0 {
		if certificate, ok := payload["certificate"].(map[string]any); ok {
			certificate["validity-duration"] = flattened.Certificate.ValidityDuration.String()
		}
	}

	// The JSON round trip normalizes every number into the float64 form Kubernetes'
	// unstructured deep-copy machinery can carry.
	normalized, err := jsonPayload(payload)
	if err != nil {
		return nil, err
	}

	return normalized.(map[string]any), nil
}

// jsonPayload converts a value with JSON tags into the JSON-native shapes the emitted manifests
// carry, so the manifest is exactly what the c9s API server accepts.
func jsonPayload(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var normalized any

	err = json.Unmarshal(raw, &normalized)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

// renderNodes renders the Node objects for the compiled topology, sorted by name. The emitted
// node names are the containerlab node names, sanitized only where Kubernetes cannot carry them
// -- the namespace is the topology boundary. Node-keyed policy on the Topology is written against
// the definition's names, so it is looked up by the node's source name.
func renderNodes(
	input *Input,
	compiled *CompiledTopology,
) ([]unstructured.Unstructured, error) {
	nodeNames := make([]string, 0, len(compiled.Nodes))
	for nodeName := range compiled.Nodes {
		nodeNames = append(nodeNames, nodeName)
	}

	sort.Strings(nodeNames)

	nodes := make([]unstructured.Unstructured, 0, len(compiled.Nodes))

	for _, nodeName := range nodeNames {
		nodeDefinition := compiled.Nodes[nodeName]
		sourceName := compiled.SourceNodeName(nodeName)
		profileName := nodeProfileNameForNode(input, compiled, nodeName)

		definition, err := definitionPayload(nodeDefinition)
		if err != nil {
			return nil, fmt.Errorf("node %q: %w", nodeName, err)
		}

		spec := maps.Clone(definition)

		if profileName != "" {
			spec["profileRef"] = map[string]any{"name": profileName}
		}

		if files := input.Deployment.FilesFromConfigMap[sourceName]; len(files) != 0 {
			raw, err := jsonPayload(files)
			if err != nil {
				return nil, fmt.Errorf("node %q: %w", nodeName, err)
			}

			spec["filesFromConfigMap"] = raw
		}

		if files := input.Deployment.FilesFromSecret[sourceName]; len(files) != 0 {
			raw, err := jsonPayload(files)
			if err != nil {
				return nil, fmt.Errorf("node %q: %w", nodeName, err)
			}

			spec["filesFromSecret"] = raw
		}

		if files := input.Deployment.FilesFromURL[sourceName]; len(files) != 0 {
			raw, err := jsonPayload(files)
			if err != nil {
				return nil, fmt.Errorf("node %q: %w", nodeName, err)
			}

			spec["filesFromURL"] = raw
		}

		if protocols := compiled.AppProtocols[nodeName]; len(protocols) != 0 {
			entries := make([]map[string]any, 0, len(protocols))
			for _, protocol := range protocols {
				entries = append(entries, map[string]any{
					"port":        protocol.Port,
					"appProtocol": protocol.AppProtocol,
				})
			}

			spec["appProtocols"] = entries
		}

		metadata := ownedObjectMetadata(input, nodeName)

		// Retain this label for human selection and compatibility; profile attachment is
		// explicit.
		labels := metadata["labels"].(map[string]any)
		labels[labelNode] = nodeName

		// The containerlab group name is organizational metadata; it rides along as a label so
		// operators can select by it, exactly as they would filter by group in containerlab.
		if nodeDefinition.Group != "" {
			labels[labelGroup] = nodeDefinition.Group
		}

		// containerlab node labels are kubernetes labels here rather than docker labels on the
		// node container. The compiler has already dropped any that kubernetes would reject or
		// that sit in c9s' own namespace, so nothing here can shadow the labels above.
		for key, value := range nodeDefinition.Labels {
			labels[key] = value
		}

		nodes = append(nodes, unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       nodeKind,
			"metadata":   metadata,
			"spec":       spec,
		}})
	}

	return nodes, nil
}

// nodeProfileNameForNode resolves the NodeProfile a Node references. A Node with distinct
// profile policy gets a dedicated profile named after it; every other Node shares the
// topology's profile.
func nodeProfileNameForNode(
	input *Input,
	compiled *CompiledTopology,
	nodeName string,
) string {
	if hasDistinctProfilePolicy(input, compiled.SourceNodeName(nodeName)) {
		return SafeConcatNameKubernetes(input.Name, nodeName)
	}

	return input.Name
}

// hasDistinctProfilePolicy reports whether a Node needs a dedicated NodeProfile. Today the
// only per-node profile policy exposed by Topology is deployment.resources; payload maps are
// rendered directly onto Nodes and therefore do not create one-off profiles.
func hasDistinctProfilePolicy(input *Input, nodeName string) bool {
	nodeResources, hasNodeResources := input.Deployment.Resources[nodeName]
	if !hasNodeResources {
		return false
	}

	defaultResources, hasDefaultResources := input.Deployment.Resources[defaultTopoKey]
	if !hasDefaultResources {
		// The map entry is itself meaningful, including an explicitly empty resource policy.
		return true
	}

	return !apiequality.Semantic.DeepEqual(nodeResources, defaultResources)
}

// renderLinks renders the Link objects for the compiled topology, sorted by name.
func renderLinks(
	input *Input,
	compiled *CompiledTopology,
) ([]unstructured.Unstructured, error) {
	links := make([]unstructured.Unstructured, 0, len(compiled.Links))

	ordered := slices.Clone(compiled.Links)
	sort.Slice(ordered, func(i, j int) bool {
		return LinkResourceName(ordered[i].EndpointA, ordered[i].EndpointB) <
			LinkResourceName(ordered[j].EndpointA, ordered[j].EndpointB)
	})

	for _, compiledLink := range ordered {
		mtu, err := jsonPayload(compiledLink.MTU)
		if err != nil {
			return nil, err
		}

		links = append(links, unstructured.Unstructured{Object: map[string]any{
			"apiVersion": c9sAPIVersion,
			"kind":       linkKind,
			"metadata": ownedObjectMetadata(
				input,
				LinkResourceName(compiledLink.EndpointA, compiledLink.EndpointB),
			),
			"spec": map[string]any{
				"endpointA": map[string]any{
					"nodeName":      compiledLink.EndpointA.NodeName,
					"interfaceName": compiledLink.EndpointA.InterfaceName,
				},
				"endpointB": map[string]any{
					"nodeName":      compiledLink.EndpointB.NodeName,
					"interfaceName": compiledLink.EndpointB.InterfaceName,
				},
				"mtu": mtu,
			},
		}})
	}

	return links, nil
}

// renderNodeProfiles renders one shared topology NodeProfile plus a complete dedicated
// profile for each compiled Node with distinct profile policy.
func renderNodeProfiles(
	input *Input,
	compiled *CompiledTopology,
) ([]unstructured.Unstructured, error) {
	perNodeNames := make([]string, 0)
	sharedProfileNeeded := false

	for nodeName := range compiled.Nodes {
		if !hasDistinctProfilePolicy(input, compiled.SourceNodeName(nodeName)) {
			sharedProfileNeeded = true

			continue
		}

		perNodeNames = append(perNodeNames, nodeName)
	}

	sort.Strings(perNodeNames)

	profiles := make([]unstructured.Unstructured, 0, len(perNodeNames)+1)

	if sharedProfileNeeded {
		profile, err := renderTopologyNodeProfile(input, compiled, input.Name)
		if err != nil {
			return nil, err
		}

		profiles = append(profiles, profile)
	}

	for _, nodeName := range perNodeNames {
		profile, err := renderTopologyNodeProfile(
			input,
			compiled,
			SafeConcatNameKubernetes(input.Name, nodeName),
		)
		if err != nil {
			return nil, err
		}

		if nodeResources, ok := input.Deployment.
			Resources[compiled.SourceNodeName(nodeName)]; ok {
			raw, err := jsonPayload(nodeResources)
			if err != nil {
				return nil, fmt.Errorf("node profile %q: %w", profile.GetName(), err)
			}

			spec := profile.Object["spec"].(map[string]any)
			spec["resources"] = raw
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// renderTopologyNodeProfile compiles topology-wide policy into a NodeProfile. Only values
// represented by the Topology API are copied; omitted values continue to inherit Config
// defaults.
func renderTopologyNodeProfile(
	input *Input,
	compiled *CompiledTopology,
	profileName string,
) (unstructured.Unstructured, error) {
	spec := map[string]any{}

	expose := map[string]any{
		"disableAutoExpose":      input.Expose.DisableAutoExpose,
		"useNodeMgmtIpv4Address": input.Expose.UseNodeMgmtIpv4Address,
		"useNodeMgmtIpv6Address": input.Expose.UseNodeMgmtIpv6Address,
	}
	if input.Expose.ExposeType != "" {
		expose["exposeType"] = input.Expose.ExposeType
	}

	spec["expose"] = expose

	if input.ImagePull.Policy != "" || len(input.ImagePull.PullSecrets) != 0 {
		imagePull := map[string]any{}

		if input.ImagePull.Policy != "" {
			imagePull["policy"] = input.ImagePull.Policy
		}

		if len(input.ImagePull.PullSecrets) != 0 {
			raw, err := jsonPayload(input.ImagePull.PullSecrets)
			if err != nil {
				return unstructured.Unstructured{}, err
			}

			imagePull["pullSecrets"] = raw
		}

		spec["imagePull"] = imagePull
	}

	if defaultResources, ok := input.Deployment.Resources[defaultTopoKey]; ok {
		raw, err := jsonPayload(defaultResources)
		if err != nil {
			return unstructured.Unstructured{}, err
		}

		spec["resources"] = raw
	}

	if scheduling := input.Deployment.Scheduling; scheduling != nil &&
		(scheduling.NodeSelector != nil || scheduling.Tolerations != nil ||
			scheduling.Affinity != nil) {
		raw, err := jsonPayload(scheduling)
		if err != nil {
			return unstructured.Unstructured{}, err
		}

		spec["scheduling"] = raw
	}

	if persistence := input.Deployment.Persistence; persistence != nil && persistence.Enabled {
		raw, err := jsonPayload(persistence)
		if err != nil {
			return unstructured.Unstructured{}, err
		}

		spec["deployment"] = map[string]any{"persistence": raw}
	}

	// Probe policy is matched against Node object names, so the node names the author wrote have
	// to follow the rename the compiler made.
	spec["statusProbes"] = compiledStatusProbes(input, compiled)

	if compiled.Mgmt != nil {
		spec["mgmt"] = map[string]any{
			"ipv4-subnet": compiled.Mgmt.IPv4Subnet,
			"ipv4-gw":     compiled.Mgmt.IPv4Gw,
			"ipv4-range":  compiled.Mgmt.IPv4Range,
			"ipv6-subnet": compiled.Mgmt.IPv6Subnet,
			"ipv6-gw":     compiled.Mgmt.IPv6Gw,
			"ipv6-range":  compiled.Mgmt.IPv6Range,
		}
	}

	if input.DisableManagement {
		spec["mgmt"] = map[string]any{"disabled": true}
	}

	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": c9sAPIVersion,
		"kind":       nodeProfile,
		"metadata":   ownedObjectMetadata(input, profileName),
		"spec":       spec,
	}}, nil
}

// compiledStatusProbes copies the topology's probe policy with every node name it holds mapped
// onto the compiled node name the Node objects carry.
func compiledStatusProbes(
	input *Input,
	compiled *CompiledTopology,
) map[string]any {
	statusProbes := map[string]any{"enabled": input.StatusProbes.Enabled}

	if input.StatusProbes.ProbeConfiguration != (ProbeConfiguration{}) {
		raw, err := jsonPayload(input.StatusProbes.ProbeConfiguration)
		if err == nil {
			statusProbes["probeConfiguration"] = raw
		}
	}

	if len(input.StatusProbes.NodeProbeConfigurations) != 0 {
		configurations := map[string]any{}

		for nodeName, probeConfiguration := range input.StatusProbes.NodeProbeConfigurations {
			if compiledName, renamed := renameSourceToCompiled(compiled, nodeName); renamed {
				nodeName = compiledName
			}

			raw, err := jsonPayload(probeConfiguration)
			if err != nil {
				continue
			}

			configurations[nodeName] = raw
		}

		statusProbes["nodeProbeConfigurations"] = configurations
	}

	if len(input.StatusProbes.ExcludedNodes) != 0 {
		excluded := make([]any, 0, len(input.StatusProbes.ExcludedNodes))

		for _, nodeName := range input.StatusProbes.ExcludedNodes {
			if compiledName, renamed := renameSourceToCompiled(compiled, nodeName); renamed {
				nodeName = compiledName
			}

			excluded = append(excluded, nodeName)
		}

		statusProbes["excludedNodes"] = excluded
	}

	return statusProbes
}

func renameSourceToCompiled(compiled *CompiledTopology, sourceName string) (string, bool) {
	for compiledName, candidate := range compiled.NodeNameSources {
		if candidate == sourceName {
			return compiledName, true
		}
	}

	return sourceName, false
}

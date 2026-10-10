// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	clabtypes "github.com/srl-labs/containerlab/types"
	yamlv3 "gopkg.in/yaml.v3"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	unknownFieldMatchCount = 3
	yamlMappingPairSize    = 2
	importedNodeLayerCount = 4

	// maxPort is the highest valid tcp/udp port number.
	maxPort = 65535
	// tcpProtocol is the default port protocol.
	tcpProtocol = "TCP"
	// udpProtocol is the udp port protocol.
	udpProtocol = "UDP"

	// labelExposePorts is a definition-only containerlab label that declares destination ports
	// to expose through the c9s Service layer.
	labelExposePorts = labelPrefix + "/exposePorts"
	// labelAppProtocols is a definition-only containerlab label that declares per-destination
	// port application protocols.
	labelAppProtocols = labelPrefix + "/appProtocols"

	// networkModeContainerPrefix is the prefix of the `network-mode` node setting expressing
	// that a node shares the network namespace of another (containerlab) node.
	networkModeContainerPrefix = "container:"

	linkEndpointElementCount = 2
)

var (
	// errParse marks malformed input.
	errParse = errors.New("parse error")
	// errInvalidData marks structurally impossible input.
	errInvalidData = errors.New("invalid data")
	// errMultipleManagementNetworks rejects a topology declaring more than one management
	// network: c9s realizes one cluster-agnostic management network per namespace, and a
	// per-network management plan is planned for a later release.
	errMultipleManagementNetworks = errors.New(
		"mgmt: c9s supports a single management network; multiple management networks are " +
			"planned for a later c9s release",
	)
	// errInvalidManagementForm marks a management block that is neither a mapping nor a sequence.
	errInvalidManagementForm = errors.New(
		"mgmt: expected a management network mapping or a list of networks",
	)
	// errInvalidKindSpecificWrapper marks a kind-specific config wrapper that is not a mapping of
	// kind config keys.
	errInvalidKindSpecificWrapper = errors.New(
		"kind-specific-config must be a mapping of kind config keys",
	)
)

// definitionFile is the strict parse of a containerlab lab definition. The node vocabulary is
// containerlab's own, so unknown fields fail exactly as any containerlab deploy would; the
// vocabulary c9s cannot carry is rejected by the compile policy below.
type definitionFile struct {
	Name     string                 `yaml:"name"`
	Prefix   *string                `yaml:"prefix,omitempty"`
	Mgmt     clabtypes.MgmtNetworks `yaml:"mgmt,omitempty"`
	Topology *fileTopology          `yaml:"topology,omitempty"`
	Debug    bool                   `yaml:"debug"`
}

// fileTopology mirrors clabtypes.Topology with the compiler's own link definitions, so an
// unsupported link type reaches the link policy below rather than failing the parse.
type fileTopology struct {
	Defaults *clabtypes.NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*clabtypes.NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*clabtypes.NodeDefinition `yaml:"nodes,omitempty"`
	Groups   map[string]*clabtypes.NodeDefinition `yaml:"groups,omitempty"`
	Links    []*linkDefinition                    `yaml:"links,omitempty"`
}

// importedNodeTopology deliberately omits Links: the compiler parses links itself, while this
// projection exists only to invoke containerlab's node inheritance behavior.
type importedNodeTopology struct {
	Defaults *clabtypes.NodeDefinition            `yaml:"defaults,omitempty"`
	Kinds    map[string]*clabtypes.NodeDefinition `yaml:"kinds,omitempty"`
	Nodes    map[string]*clabtypes.NodeDefinition `yaml:"nodes,omitempty"`
	Groups   map[string]*clabtypes.NodeDefinition `yaml:"groups,omitempty"`
}

// compileContainerlabDefinition compiles a containerlab topology definition: every node gets
// the topology defaults and its kind expanded *into* its definition (so the emitted Node
// objects are self contained), and the links section becomes the compiled wire list.
func compileContainerlabDefinition(
	logger Logger,
	input *Input,
	diagnostics *compileDiagnostics,
) (*CompiledTopology, error) {
	definition, unknownFields, err := loadDefinitionFile(input.Definition)
	if err != nil {
		logger.Errorf("failed parsing containerlab config, error: %s", err)

		return nil, err
	}

	for _, unknownField := range unknownFields {
		diagnostics.add(diagnosticFromUnknownField(unknownField))
	}

	nodeFieldLines, err := definitionFieldLines(input.Definition)
	if err != nil {
		return nil, err
	}

	validateRejectedNodeFields(nodeFieldLines, diagnostics)

	validateAbsorbedNodeVocabulary(definition.Topology, nodeFieldLines, diagnostics)

	if definition.Topology == nil {
		return nil, fmt.Errorf(
			"%w: containerlab definition has no topology section",
			errParse,
		)
	}

	importedTopology, err := importedTopology(definition.Topology)
	if err != nil {
		return nil, err
	}

	if len(definition.Mgmt) > 1 {
		return nil, fmt.Errorf("%w: %d were declared",
			errMultipleManagementNetworks, len(definition.Mgmt))
	}

	managementFieldLines, err := topLevelMappingFieldLines(input.Definition, "mgmt")
	if err != nil {
		return nil, err
	}

	validateManagementPolicy(managementFieldLines, diagnostics)

	compiled := &CompiledTopology{
		Kind: TopologyKindContainerlab,
		Nodes: make(
			map[string]*clabtypes.NodeDefinition,
			len(definition.Topology.Nodes),
		),
		AppProtocols: make(
			map[string][]AppProtocol,
			len(definition.Topology.Nodes),
		),
	}

	if len(definition.Mgmt) == 1 {
		compiled.Mgmt = definition.Mgmt[0]
	}

	nodeNames := make([]string, 0, len(definition.Topology.Nodes))
	for nodeName := range definition.Topology.Nodes {
		nodeNames = append(nodeNames, nodeName)
	}

	sort.Strings(nodeNames)

	for _, nodeName := range nodeNames {
		compiled.Nodes[nodeName], err = flattenNodeDefinition(
			importedTopology,
			nodeName,
			nodeFieldLines,
			diagnostics,
		)
		if err != nil {
			return nil, err
		}

		normalizeNodePorts(diagnostics, nodeName, compiled.Nodes[nodeName])
		consumeExposePortsLabel(diagnostics, nodeName, compiled.Nodes[nodeName])
		appProtocols := consumeAppProtocolsLabel(diagnostics, nodeName, compiled.Nodes[nodeName])
		if len(appProtocols) != 0 {
			compiled.AppProtocols[nodeName] = appProtocols
		}
		dropUnusableNodeLabels(diagnostics, nodeName, compiled.Nodes[nodeName])
	}

	validateNodeNetworkModes(compiled.Nodes, diagnostics)
	validateNodeVocabularyPolicies(compiled.Nodes, diagnostics)
	validateNodeBinds(compiled.Nodes, diagnostics)
	validateNodeAliases(compiled.Nodes, diagnostics)

	compiled.Links, err = compileContainerlabLinks(definition.Topology, diagnostics)
	if err != nil {
		logger.Errorf("failed compiling containerlab links, error: %s", err)

		return nil, err
	}

	// Renaming last keeps every diagnostic above pointing at the node names the definition
	// actually writes.
	sanitizeCompiledNodeNames(logger, compiled, diagnostics)

	for _, warning := range diagnostics.warnings() {
		logger.Warnf("topology compile: %s", formatDiagnostic(warning))
	}

	diagnosticErr := diagnostics.err()
	if diagnosticErr != nil {
		return nil, diagnosticErr
	}

	return compiled, nil
}

// validateAbsorbedNodeVocabulary rejects keys that containerlab's generic node definition
// declares riding inside an explicit `kind-specific-config` wrapper: since the kind-specific
// config mechanism absorbs unknown keys, a generic field like `stages` would otherwise ride into
// the kind as kind-owned config instead of failing the compile with the structured rejection it
// documents. The check walks every defaults, kinds, groups and nodes block -- including blocks
// no node uses -- so a mistake fails the compile even when the block is not referenced.
func validateAbsorbedNodeVocabulary(
	topology *fileTopology,
	fieldLines map[string]map[string]int,
	diagnostics *compileDiagnostics,
) {
	if topology == nil {
		return
	}

	vocabulary := importedNodeVocabulary()

	blocks := []struct {
		from        string
		definitions []*clabtypes.NodeDefinition
	}{
		{from: "defaults", definitions: singleDefinition(topology.Defaults)},
	}

	for _, kindName := range sortedVocabularyBlockNames(topology.Kinds) {
		blocks = append(
			blocks,
			struct {
				from        string
				definitions []*clabtypes.NodeDefinition
			}{
				from:        "kinds." + kindName,
				definitions: singleDefinition(topology.Kinds[kindName]),
			},
		)
	}

	for _, groupName := range sortedVocabularyBlockNames(topology.Groups) {
		blocks = append(
			blocks,
			struct {
				from        string
				definitions []*clabtypes.NodeDefinition
			}{
				from:        "groups." + groupName,
				definitions: singleDefinition(topology.Groups[groupName]),
			},
		)
	}

	for _, nodeName := range sortedVocabularyBlockNames(topology.Nodes) {
		blocks = append(
			blocks,
			struct {
				from        string
				definitions []*clabtypes.NodeDefinition
			}{
				from:        "nodes." + nodeName,
				definitions: singleDefinition(topology.Nodes[nodeName]),
			},
		)
	}

	for _, block := range blocks {
		for _, definition := range block.definitions {
			for _, key := range sortedVocabularyBlockNames(definition.KindSpecificConfig) {
				if !vocabulary[key] {
					continue
				}

				diagnostics.add(absorbedGenericFieldDiagnostic(
					key,
					fieldLines[block.from][key],
				))
			}
		}
	}
}

func singleDefinition(
	definition *clabtypes.NodeDefinition,
) []*clabtypes.NodeDefinition {
	if definition == nil {
		return nil
	}

	return []*clabtypes.NodeDefinition{definition}
}

func sortedVocabularyBlockNames[V any](values map[string]V) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// absorbedGenericFieldDiagnostic reports a node key that containerlab's generic node definition
// declares but the compile cannot carry as kind-owned config, mirroring the strict parser's
// unknown-field diagnostics.
func absorbedGenericFieldDiagnostic(
	key string,
	line int,
) Diagnostic {
	diagnostic := Diagnostic{Code: "unsupported-field", Message: fmt.Sprintf(
		"field %s is not supported by clabernetes",
		key,
	)}

	diagnostic.Path = key
	diagnostic.Line = line

	if reason, rejected := rejectedContainerlabFieldReason(key); rejected {
		diagnostic.Message = fmt.Sprintf(
			"field %q is rejected: %s",
			key,
			reason,
		)
	}

	return diagnostic
}

// loadDefinitionFile strictly parses the definition with containerlab's own vocabulary. Unknown
// fields are returned separately as yaml error lines so the compile can reject them with the
// structured diagnostics it documents, while every decodable field still reaches the compiler.
// Node definitions absorb unknown keys into their kind-specific config the way containerlab's
// own decoder does, so only fields outside a node definition reach the strict check here.
func loadDefinitionFile(definition string) (*definitionFile, []string, error) {
	file := &definitionFile{}

	decoder := yamlv3.NewDecoder(strings.NewReader(definition))
	decoder.KnownFields(true)

	err := decoder.Decode(file)
	if err == nil {
		return file, nil, nil
	}

	var typeErr *yamlv3.TypeError
	if !errors.As(err, &typeErr) {
		return nil, nil, err
	}

	unknownFields := make([]string, 0, len(typeErr.Errors))

	for _, entry := range typeErr.Errors {
		if !strings.Contains(entry, "not found in type") {
			// A genuine type error must not be swallowed by the vocabulary policy.
			return nil, nil, fmt.Errorf("%s", entry)
		}

		unknownFields = append(unknownFields, entry)
	}

	return file, unknownFields, nil
}

// validateRejectedNodeFields rejects the containerlab fields the c9s vocabulary deliberately
// does not carry, naming why. The strict parser above already fails on fields containerlab does
// not declare; this check covers the fields containerlab knows but the direct runtime cannot
// realize, and it runs on the raw yaml so an explicitly declared field is caught even when its
// value would otherwise be indistinguishable from unset.
func validateRejectedNodeFields(
	fieldLines map[string]map[string]int,
	diagnostics *compileDiagnostics,
) {
	for _, block := range sortedVocabularyBlocks(fieldLines) {
		for _, key := range slices.Sorted(maps.Keys(fieldLines[block])) {
			reason, rejected := rejectedContainerlabFieldReason(key)
			if !rejected {
				continue
			}

			diagnostics.add(Diagnostic{
				Code:    "unsupported-field",
				Path:    key,
				Line:    fieldLines[block][key],
				Message: fmt.Sprintf("field %q is rejected: %s", key, reason),
			})
		}
	}
}

// sortedVocabularyBlocks lists the defaults, kinds, groups and nodes blocks present in the
// field-lines map in deterministic order.
func sortedVocabularyBlocks(fieldLines map[string]map[string]int) []string {
	blocks := make([]string, 0, len(fieldLines))
	for block := range fieldLines {
		blocks = append(blocks, block)
	}

	sort.Strings(blocks)

	return blocks
}

// rejectedContainerlabFieldReason documents baseline vocabulary the direct runtime deliberately
// does not accept: the strict parser already fails on these fields, and the returned message
// replaces the raw parse error so the diagnostic states why the field is rejected rather than
// implying it is unknown.
func rejectedContainerlabFieldReason(field string) (string, bool) {
	switch field {
	case "runtime":
		return "container runtime selection is Docker-only; " +
			"direct device Pods always use the cluster's container runtime", true
	case "auto-remove":
		return "Docker auto-remove has no direct-Pod equivalent; " +
			"Kubernetes owns container lifecycle", true
	case "pid-mode":
		return "Docker PID-namespace sharing has no direct-Pod mapping", true
	case "cgroupns-mode":
		return "Docker cgroup-namespace selection has no direct-Pod mapping", true
	case "cpu-set":
		return "CPU pinning has no portable Pod mapping", true
	case "stages":
		return "containerlab deployment stages coordinate multi-node boot ordering " +
			"the direct runtime does not implement", true
	case "credentials":
		return "node credentials are not represented; " +
			"imported kind default credentials still apply", true
	default:
		return "", false
	}
}

var unknownFieldPattern = regexp.MustCompile(`^line (\d+): field ([^ ]+)`)

func diagnosticFromUnknownField(message string) Diagnostic {
	diagnostic := Diagnostic{
		Code:    "unsupported-field",
		Message: message,
	}

	matches := unknownFieldPattern.FindStringSubmatch(message)
	if len(matches) != unknownFieldMatchCount {
		return diagnostic
	}

	diagnostic.Line, _ = strconv.Atoi(matches[1])
	diagnostic.Path = matches[2]

	if reason, rejected := rejectedContainerlabFieldReason(diagnostic.Path); rejected {
		diagnostic.Message = fmt.Sprintf(
			"field %q is rejected: %s",
			diagnostic.Path,
			reason,
		)
	}

	return diagnostic
}

func importedTopology(topology *fileTopology) (*clabtypes.Topology, error) {
	if topology == nil {
		return nil, fmt.Errorf("%w: containerlab definition has no topology section", errParse)
	}

	imported := &clabtypes.Topology{
		Defaults: topology.Defaults,
		Kinds:    topology.Kinds,
		Nodes:    topology.Nodes,
		Groups:   topology.Groups,
	}
	if imported.Defaults == nil {
		imported.Defaults = &clabtypes.NodeDefinition{}
	}

	return imported, nil
}

// flattenNodeDefinition asks containerlab's Topology for every supported effective value. c9s
// owns the primitive shape, but it does not duplicate containerlab's inheritance rules; the
// flattened definition stays in containerlab vocabulary and only the fields the c9s vocabulary
// carries are populated.
func flattenNodeDefinition(
	topology *clabtypes.Topology,
	nodeName string,
	fieldLines map[string]map[string]int,
	diagnostics *compileDiagnostics,
) (*clabtypes.NodeDefinition, error) {
	flattened := &clabtypes.NodeDefinition{
		Kind:          topology.GetNodeKind(nodeName),
		Group:         topology.GetNodeGroup(nodeName),
		Type:          topology.GetNodeType(nodeName),
		Image:         topology.GetNodeImage(nodeName),
		License:       topology.GetNodeLicense(nodeName),
		StartupConfig: topology.GetNodeStartupConfig(nodeName),
		StartupDelay:  topology.GetNodeStartupDelay(nodeName),
		RestartPolicy: topology.GetRestartPolicy(nodeName),
		Entrypoint:    topology.GetNodeEntrypoint(nodeName),
		Cmd:           topology.GetNodeCmd(nodeName),
		Exec:          slices.Clone(topology.GetNodeExec(nodeName)),
		User:          topology.GetNodeUser(nodeName),
		Devices:       slices.Clone(topology.GetNodeDevices(nodeName)),
		CapAdd:        slices.Clone(topology.GetNodeCapAdd(nodeName)),
		SecurityOpts:  slices.Clone(topology.GetNodeSecurityOpts(nodeName)),
		Tmpfs:         maps.Clone(topology.GetNodeTmpfs(nodeName)),
		ShmSize:       topology.GetNodeShmSize(nodeName),
		CPU:           topology.GetNodeCPU(nodeName),
		Memory:        topology.GetNodeMemory(nodeName),
		LinkApplyMode: topology.GetNodeLinkApplyMode(nodeName),
		Ports:         importedNodePorts(topology, nodeName),
		NetworkMode:   topology.GetNodeNetworkMode(nodeName),
		Env:           maps.Clone(topology.GetNodeEnv(nodeName)),
		EnvFiles:      slices.Clone(topology.GetNodeEnvFiles(nodeName)),
		Sysctls:       maps.Clone(topology.GetSysCtl(nodeName)),
		Labels:        maps.Clone(topology.GetNodeLabels(nodeName)),
	}

	layers := importedNodeLayers(topology, nodeName)
	// The imported image-pull-policy getter normalizes its result, which would make every
	// flattened Node look explicitly configured; the raw most-specific declaration preserves
	// the Node-versus-default distinction the planner needs.
	flattened.ImagePullPolicy = firstImportedString(
		layers,
		func(definition *clabtypes.NodeDefinition) string { return definition.ImagePullPolicy },
	)
	flattened.EnforceStartupConfig = clonePointer(firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *bool {
			return definition.EnforceStartupConfig
		},
	))
	flattened.SuppressStartupConfig = clonePointer(firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *bool {
			return definition.SuppressStartupConfig
		},
	))
	flattened.Privileged = clonePointer(firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *bool { return definition.Privileged },
	))

	binds, err := topology.GetNodeBinds(nodeName)
	if err != nil {
		return nil, err
	}

	flattened.Binds = slices.Clone(binds)

	if sourceNode := topology.Nodes[nodeName]; sourceNode != nil {
		// Containerlab intentionally does not inherit per-node management addresses or
		// network aliases.
		flattened.MgmtIPv4 = sourceNode.MgmtIPv4
		flattened.MgmtIPv6 = sourceNode.MgmtIPv6
		flattened.Aliases = slices.Clone(sourceNode.Aliases)
	}

	if firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *clabtypes.ConfigDispatcher {
			return definition.Config
		},
	) != nil {
		flattened.Config = clonePointer(topology.GetNodeConfigDispatcher(nodeName))
	}

	if firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *clabtypes.DNSConfig {
			return definition.DNS
		},
	) != nil {
		flattened.DNS = clonePointer(topology.GetNodeDns(nodeName))
	}

	err = flattenNodeContracts(topology, layers, nodeName, flattened)
	if err != nil {
		return nil, err
	}

	err = collectKindSpecificConfig(
		topology,
		nodeName,
		flattened,
		fieldLines,
		diagnostics,
	)
	if err != nil {
		return nil, err
	}

	return flattened, nil
}

// flattenNodeContracts resolves the pointer-valued health and certificate contracts through the
// inherited inheritance rules onto the flattened node.
func flattenNodeContracts(
	topology *clabtypes.Topology,
	layers []*clabtypes.NodeDefinition,
	nodeName string,
	flattened *clabtypes.NodeDefinition,
) error {
	if firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *clabtypes.HealthcheckConfig {
			return definition.HealthCheck
		},
	) != nil {
		flattened.HealthCheck = clonePointer(topology.GetHealthCheckConfig(nodeName))
	}

	if firstImportedPointer(
		layers,
		func(definition *clabtypes.NodeDefinition) *clabtypes.CertificateConfig {
			return definition.Certificate
		},
	) != nil {
		certificate := topology.GetCertificateConfig(nodeName)

		flattened.Certificate = &clabtypes.CertificateConfig{
			Issue:   clonePointer(certificate.Issue),
			KeySize: certificate.KeySize,
			SANs:    slices.Clone(certificate.SANs),
		}
		if certificate.ValidityDuration > 0 {
			flattened.Certificate.ValidityDuration = certificate.ValidityDuration
		}
	}

	return nil
}

func importedNodeLayers(
	topology *clabtypes.Topology,
	nodeName string,
) []*clabtypes.NodeDefinition {
	layers := make([]*clabtypes.NodeDefinition, 0, importedNodeLayerCount)
	if node := topology.Nodes[nodeName]; node != nil {
		layers = append(layers, node)
	}

	if group := topology.GetGroup(topology.GetNodeGroup(nodeName)); group != nil {
		layers = append(layers, group)
	}

	if kind := topology.GetKind(topology.GetNodeKind(nodeName)); kind != nil {
		layers = append(layers, kind)
	}

	layers = append(layers, topology.GetDefaults())

	return layers
}

func firstImportedPointer[T any](
	layers []*clabtypes.NodeDefinition,
	get func(*clabtypes.NodeDefinition) *T,
) *T {
	for _, layer := range layers {
		if value := get(layer); value != nil {
			return value
		}
	}

	return nil
}

func firstImportedString(
	layers []*clabtypes.NodeDefinition,
	get func(*clabtypes.NodeDefinition) string,
) string {
	for _, layer := range layers {
		if value := get(layer); value != "" {
			return value
		}
	}

	return ""
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}

	cloned := *value

	return &cloned
}

// importedNodePorts retains the package's most-specific non-empty ports rule while preserving the
// original strings so strict diagnostics can still name Docker host-side pinning exactly.
func importedNodePorts(topology *clabtypes.Topology, nodeName string) []string {
	for _, layer := range importedNodeLayers(topology, nodeName) {
		if len(layer.Ports) != 0 {
			return slices.Clone(layer.Ports)
		}
	}

	return nil
}

// kindSpecificComponentsKey is the one kind-specific config key c9s carries as a typed field
// instead of the generic passthrough: the component inventory drives c9s' own device rendering
// (chassis component DNS aliases), and the Node CRD exposes it as a validated, typed vocabulary.
const kindSpecificComponentsKey = "components"

// kindSpecificWrapperKey is the yaml key c9s' Node vocabulary uses to carry merged kind-specific
// config. A definition may write it explicitly; the compiler unwraps it so the same key never
// appears as kind-owned config.
const kindSpecificWrapperKey = "kind-specific-config"

// collectKindSpecificConfig carries the node's merged kind-specific config keys onto the
// flattened definition, mirroring containerlab's kind-specific config model: keys are merged over
// the regular inheritance order by containerlab, and the kind validates them strictly when the
// node is planned. The `components` key is carried by the typed Components field and is never
// duplicated into the passthrough, and the `mgmt-net` key is accepted with a diagnostic because
// c9s realizes a single management network per namespace.
func collectKindSpecificConfig(
	topology *clabtypes.Topology,
	nodeName string,
	flattened *clabtypes.NodeDefinition,
	fieldLines map[string]map[string]int,
	diagnostics *compileDiagnostics,
) error {
	if mgmtNet := topology.GetNodeMgmtNet(nodeName); mgmtNet != "" {
		flattened.MgmtNet = mgmtNet

		diagnostics.add(Diagnostic{
			Code: "ignored-node-field",
			Path: "topology.nodes." + nodeName + ".mgmt-net",
			Line: fieldLines["nodes."+nodeName]["mgmt-net"],
			Message: fmt.Sprintf(
				"node %q mgmt-net %q is accepted and ignored; c9s realizes a single management "+
					"network per namespace",
				nodeName,
				mgmtNet,
			),
			Warning: true,
		})
	}

	entries := topology.GetNodeKindSpecificConfig(nodeName)
	if len(entries) == 0 {
		return nil
	}

	if flattened.KindSpecificConfig == nil {
		flattened.KindSpecificConfig = make(map[string]any, len(entries))
	}

	for _, entry := range entries {
		switch entry.Key {
		case kindSpecificWrapperKey:
			wrapper := kindSpecificWrapperEntries(entry.Value)
			if wrapper == nil {
				return fmt.Errorf("node %q: %w", nodeName, errInvalidKindSpecificWrapper)
			}

			for keyText, wrapperValue := range wrapper {
				if err := carryKindSpecificValue(
					flattened,
					nodeName,
					keyText,
					wrapperValue,
					fieldLines,
					entry.From,
					diagnostics,
				); err != nil {
					return err
				}
			}

			continue
		}

		if err := carryKindSpecificValue(
			flattened,
			nodeName,
			entry.Key,
			entry.Value,
			fieldLines,
			entry.From,
			diagnostics,
		); err != nil {
			return err
		}
	}

	return nil
}

// kindSpecificWrapperEntries flattens a kind-specific config wrapper mapping into per-key
// entries. The wrapper value arrives through either yaml unmarshaler, so both mapping shapes are
// accepted.
func kindSpecificWrapperEntries(value any) map[string]any {
	switch wrapper := value.(type) {
	case map[string]any:
		return wrapper
	case map[any]any:
		entries := make(map[string]any, len(wrapper))
		for key, entryValue := range wrapper {
			keyText, ok := key.(string)
			if !ok {
				return nil
			}

			entries[keyText] = entryValue
		}

		return entries
	default:
		return nil
	}
}

// carryKindSpecificValue renders one kind-specific config value onto the flattened node and
// reports a duplicate declaration of the same key inside one node block.
func carryKindSpecificValue(
	flattened *clabtypes.NodeDefinition,
	nodeName,
	key string,
	value any,
	fieldLines map[string]map[string]int,
	from string,
	diagnostics *compileDiagnostics,
) error {
	if _, duplicated := flattened.KindSpecificConfig[key]; duplicated {
		line := 0
		if lines := fieldLines[from]; lines != nil {
			line = lines[key]
		}

		diagnostics.add(Diagnostic{
			Code: "duplicate-kind-config",
			Path: "topology." + from + "." + key,
			Line: line,
			Message: fmt.Sprintf(
				"node %q declares kind config key %q more than once",
				nodeName,
				key,
			),
		})

		return nil
	}

	flattened.KindSpecificConfig[key] = value

	return nil
}

// importedNodeVocabulary is the yaml vocabulary containerlab's generic node definition declares.
// Keys absorbed by the inline kind-specific config mapping that collide with it are generic
// fields, not kind-owned keys, and get the compiler's unsupported-field diagnostics.
var importedNodeVocabulary = sync.OnceValue(
	func() map[string]bool {
		vocabulary := map[string]bool{}

		collectImportedNodeVocabularyKeys(reflect.TypeFor[clabtypes.NodeDefinition](), vocabulary)

		return vocabulary
	},
)

func collectImportedNodeVocabularyKeys(walk reflect.Type, into map[string]bool) {
	for walk.Kind() == reflect.Pointer || walk.Kind() == reflect.Slice ||
		walk.Kind() == reflect.Array || walk.Kind() == reflect.Map {
		walk = walk.Elem()
	}

	if walk.Kind() != reflect.Struct {
		return
	}

	for field := range walk.Fields() {
		tag, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if tag == "" || tag == "-" {
			continue
		}

		into[tag] = true
	}
}

// definitionFieldLines records the source line of every field of every topology block that can
// hold node definitions, keyed by the block identity containerlab reports for merged
// kind-specific config entries: nodes.<name>, groups.<name>, kinds.<name>, or defaults.
func definitionFieldLines(definition string) (map[string]map[string]int, error) {
	document := &yamlv3.Node{}

	err := yamlv3.Unmarshal([]byte(definition), document)
	if err != nil {
		return nil, err
	}

	if len(document.Content) == 0 || document.Content[0].Kind != yamlv3.MappingNode {
		return map[string]map[string]int{}, nil
	}

	root := document.Content[0]

	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value != "topology" {
			continue
		}

		topology := root.Content[index+1]
		if topology.Kind != yamlv3.MappingNode {
			break
		}

		return mappingFieldLines(topology)
	}

	return map[string]map[string]int{}, nil
}

// mappingFieldLines walks one topology mapping and records the field lines of its defaults,
// kinds, groups, and nodes blocks.
func mappingFieldLines(topology *yamlv3.Node) (map[string]map[string]int, error) {
	lines := map[string]map[string]int{}

	for index := 0; index+1 < len(topology.Content); index += 2 {
		key, value := topology.Content[index], topology.Content[index+1]

		switch key.Value {
		case "defaults":
			if value.Kind == yamlv3.MappingNode {
				lines["defaults"] = childFieldLines(value)
			}
		case "kinds", "groups", "nodes":
			if value.Kind != yamlv3.MappingNode {
				continue
			}

			prefix := key.Value
			for nameIndex := 0; nameIndex+1 < len(value.Content); nameIndex += 2 {
				name, definition := value.Content[nameIndex], value.Content[nameIndex+1]
				if definition.Kind != yamlv3.MappingNode {
					continue
				}

				lines[prefix+"."+name.Value] = childFieldLines(definition)
			}
		}
	}

	return lines, nil
}

func childFieldLines(mapping *yamlv3.Node) map[string]int {
	lines := make(map[string]int, len(mapping.Content)/yamlMappingPairSize)
	for index := 0; index+1 < len(mapping.Content); index += yamlMappingPairSize {
		childKey := mapping.Content[index]
		lines[childKey.Value] = childKey.Line
	}

	return lines
}

// validateManagementPolicy records the management-network fields the c9s management mesh has no
// equivalent for as accepted-and-ignored warnings.
func validateManagementPolicy(
	fieldLines map[string]int,
	diagnostics *compileDiagnostics,
) {
	fields := []struct {
		name    string
		path    string
		message string
	}{
		{
			name:    "network",
			path:    "mgmt.network",
			message: "container runtime management network names are accepted and ignored",
		},
		{
			name:    "bridge",
			path:    "mgmt.bridge",
			message: "container runtime management bridges are accepted and ignored",
		},
		{
			name:    "mtu",
			path:    "mgmt.mtu",
			message: "Docker management-network MTU is accepted and ignored",
		},
		{
			name: "external-access",
			path: "mgmt.external-access",
			message: "Docker management-network external access is accepted and ignored; " +
				"exposure is governed by Kubernetes Services",
		},
		{
			name:    "skip-when-unused",
			path:    "mgmt.skip-when-unused",
			message: "conditional container runtime network creation is accepted and ignored",
		},
		{
			name:    "driver-opts",
			path:    "mgmt.driver-opts",
			message: "container runtime network driver options are accepted and ignored",
		},
		{
			name: "driver",
			path: "mgmt.driver",
			message: "management network drivers are a container runtime concept; " +
				"the c9s management mesh has no driver",
		},
		{
			name: "ipam",
			path: "mgmt.ipam",
			message: "container runtime IPAM providers are accepted and ignored; " +
				"c9s allocates management addresses itself",
		},
		{
			name: "macvlan-parent",
			path: "mgmt.macvlan-parent",
			message: "macvlan management networks are a container runtime concept and are " +
				"accepted and ignored",
		},
		{
			name: "macvlan-mode",
			path: "mgmt.macvlan-mode",
			message: "macvlan management networks are a container runtime concept and are " +
				"accepted and ignored",
		},
		{
			name: "macvlan-aux",
			path: "mgmt.macvlan-aux",
			message: "macvlan management networks are a container runtime concept and are " +
				"accepted and ignored",
		},
		{
			name: "tailscale",
			path: "mgmt.tailscale",
			message: "management network tailscale integration is a container runtime " +
				"capability and is accepted and ignored",
		},
	}
	for _, field := range fields {
		line, present := fieldLines[field.name]
		if !present {
			continue
		}

		diagnostics.add(Diagnostic{
			Code:    "ignored-management-field",
			Path:    field.path,
			Line:    line,
			Message: field.message,
			Warning: true,
		})
	}
}

func topLevelMappingFieldLines(definition, fieldName string) (map[string]int, error) {
	document := &yamlv3.Node{}

	err := yamlv3.Unmarshal([]byte(definition), document)
	if err != nil {
		return nil, err
	}

	if len(document.Content) == 0 || document.Content[0].Kind != yamlv3.MappingNode {
		return map[string]int{}, nil
	}

	root := document.Content[0]
	for index := 0; index+1 < len(root.Content); index += 2 {
		key, value := root.Content[index], root.Content[index+1]
		if key.Value != fieldName {
			continue
		}

		// The management block may be a single mapping or a sequence of networks; the compiler
		// accepts one network, so its element holds the same fields.
		if value.Kind == yamlv3.SequenceNode && len(value.Content) == 1 &&
			value.Content[0].Kind == yamlv3.MappingNode {
			value = value.Content[0]
		}

		if value.Kind != yamlv3.MappingNode {
			continue
		}

		lines := make(map[string]int, len(value.Content)/yamlMappingPairSize)
		for childIndex := 0; childIndex+1 < len(value.Content); childIndex += yamlMappingPairSize {
			childKey := value.Content[childIndex]
			lines[childKey.Value] = childKey.Line
		}

		return lines, nil
	}

	return map[string]int{}, nil
}

// validateNodeNetworkModes mirrors the Node CRD's container:<primary> contract in the compiler
// and additionally verifies references and cycles that the single-object CRD cannot see. This is
// required for strict, API-free validation and dry-run: callers must not need to create a Node
// before learning that a native host/none mode or an impossible pod group is unsupported.
func validateNodeNetworkModes(
	nodes map[string]*clabtypes.NodeDefinition,
	diagnostics *compileDiagnostics,
) {
	for _, nodeName := range sortedNodeNames(nodes) {
		networkMode := nodes[nodeName].NetworkMode
		if networkMode == "" {
			continue
		}

		primary := parseNetworkModeContainer(networkMode)

		path := fmt.Sprintf("topology.nodes.%s.network-mode", nodeName)
		// The primary is a containerlab node name, and node names Kubernetes cannot carry are
		// sanitized at the end of the compile -- so what has to hold here is that the value names
		// a node at all, not that it is already a Kubernetes name.
		if primary == "" || SanitizeName(primary) == "" {
			diagnostics.add(Diagnostic{
				Code: "unsupported-network-mode",
				Path: path,
				Message: fmt.Sprintf(
					"node %q network-mode %q is unsupported; "+
						"c9s accepts only container:<primary node name>",
					nodeName,
					networkMode,
				),
			})

			continue
		}

		if _, exists := nodes[primary]; !exists {
			diagnostics.add(Diagnostic{
				Code: "unknown-network-mode-primary",
				Path: path,
				Message: fmt.Sprintf(
					"node %q shares a pod with nonexistent primary node %q",
					nodeName,
					primary,
				),
			})

			continue
		}

		seen := map[string]bool{nodeName: true}

		current := primary
		for current != "" {
			if seen[current] {
				diagnostics.add(Diagnostic{
					Code: "network-mode-cycle",
					Path: path,
					Message: fmt.Sprintf(
						"node %q network-mode participates in a pod-group cycle",
						nodeName,
					),
				})

				break
			}

			seen[current] = true

			nextNode, exists := nodes[current]
			if !exists {
				break
			}

			current = parseNetworkModeContainer(
				nextNode.NetworkMode,
			)
		}
	}
}

// sortedNodeNames returns the compiled node names in deterministic diagnostic order.
func sortedNodeNames(nodes map[string]*clabtypes.NodeDefinition) []string {
	nodeNames := make([]string, 0, len(nodes))
	for nodeName := range nodes {
		nodeNames = append(nodeNames, nodeName)
	}

	sort.Strings(nodeNames)

	return nodeNames
}

// validateNodeVocabularyPolicies rejects recognized policy fields whose values the direct
// runtime cannot realize, so an unrepresentable declaration fails at compile rather than at
// Node admission or planning.
func validateNodeVocabularyPolicies(
	nodes map[string]*clabtypes.NodeDefinition,
	diagnostics *compileDiagnostics,
) {
	restartPolicies := map[string]bool{
		"": true, "always": true, "Always": true, "unless-stopped": true, "Unless-stopped": true,
	}
	pullPolicies := map[string]bool{
		"": true, "always": true, "Always": true, "never": true, "Never": true,
		"ifnotpresent": true, "IfNotPresent": true,
	}
	linkApplyModes := map[string]bool{"": true, "live": true, "restart": true, "recreate": true}

	for _, nodeName := range sortedNodeNames(nodes) {
		node := nodes[nodeName]

		if !restartPolicies[node.RestartPolicy] {
			diagnostics.add(Diagnostic{
				Code: "unsupported-restart-policy",
				Path: fmt.Sprintf("topology.nodes.%s.restart-policy", nodeName),
				Message: fmt.Sprintf(
					"node %q restart policy %q has no shared-Pod mapping; "+
						"direct device containers support always or unless-stopped",
					nodeName,
					node.RestartPolicy,
				),
			})
		}

		if !pullPolicies[node.ImagePullPolicy] {
			diagnostics.add(Diagnostic{
				Code: "unsupported-image-pull-policy",
				Path: fmt.Sprintf("topology.nodes.%s.image-pull-policy", nodeName),
				Message: fmt.Sprintf(
					"node %q image pull policy %q is not a containerlab pull policy "+
						"(always, never, ifnotpresent)",
					nodeName,
					node.ImagePullPolicy,
				),
			})
		}

		if !linkApplyModes[string(node.LinkApplyMode)] {
			diagnostics.add(Diagnostic{
				Code: "unsupported-link-apply-mode",
				Path: fmt.Sprintf("topology.nodes.%s.link-apply-mode", nodeName),
				Message: fmt.Sprintf(
					"node %q link apply mode %q is not live, restart, or recreate",
					nodeName,
					node.LinkApplyMode,
				),
			})
		}
	}
}

// validateNodeBinds rejects binds that land on container paths the kubelet or the direct
// runtime owns: such a bind either renders an invalid Deployment (the kubelet already mounts
// the path in every container) or silently shadows Pod-managed content. Docker containerlab
// can bind these paths because it owns the container filesystem; direct device Pods cannot.
func validateNodeBinds(
	nodes map[string]*clabtypes.NodeDefinition,
	diagnostics *compileDiagnostics,
) {
	const (
		bindMinParts = 2
		bindMaxParts = 3
	)

	for _, nodeName := range sortedNodeNames(nodes) {
		for index, bind := range nodes[nodeName].Binds {
			parts := strings.SplitN(bind, ":", bindMaxParts)
			if len(parts) < bindMinParts {
				continue
			}

			for _, half := range parts[:2] {
				reason, reserved := reservedContainerPathReason(half)
				if !reserved {
					continue
				}

				diagnostics.add(Diagnostic{
					Code: "reserved-bind-path",
					Path: fmt.Sprintf("topology.nodes.%s.binds[%d]", nodeName, index),
					Message: fmt.Sprintf(
						"node %q bind %q uses reserved container path %q: %s",
						nodeName,
						bind,
						half,
						reason,
					),
				})

				break
			}
		}
	}
}

// validateNodeAliases enforces that every alias can become a same-namespace Service name and
// that no alias collides with a node name or another alias anywhere in the topology.
func validateNodeAliases(
	nodes map[string]*clabtypes.NodeDefinition,
	diagnostics *compileDiagnostics,
) {
	owners := map[string]string{}
	for nodeName := range nodes {
		owners[nodeName] = fmt.Sprintf("node %q", nodeName)
	}

	for _, nodeName := range sortedNodeNames(nodes) {
		for index, alias := range nodes[nodeName].Aliases {
			path := fmt.Sprintf("topology.nodes.%s.aliases[%d]", nodeName, index)

			if problems := k8svalidation.IsDNS1035Label(alias); len(problems) != 0 {
				diagnostics.add(Diagnostic{
					Code: "invalid-alias",
					Path: path,
					Message: fmt.Sprintf(
						"node %q alias %q cannot become a Kubernetes Service name: %s",
						nodeName,
						alias,
						strings.Join(problems, "; "),
					),
				})

				continue
			}

			if owner, exists := owners[alias]; exists {
				diagnostics.add(Diagnostic{
					Code: "duplicate-alias",
					Path: path,
					Message: fmt.Sprintf(
						"node %q alias %q collides with %s",
						nodeName,
						alias,
						owner,
					),
				})

				continue
			}

			owners[alias] = fmt.Sprintf("node %q alias %q", nodeName, alias)
		}
	}
}

// normalizeNodePorts strips Docker-style host pinning that direct Node resources cannot
// preserve: the Pod-side port is kept, and a warning tells the author the host half was
// dropped. Host port pinning only ever described the local Docker host, so its loss cannot
// change lab behavior inside the cluster.
func normalizeNodePorts(
	diagnostics *compileDiagnostics,
	nodeName string,
	nodeDefinition *clabtypes.NodeDefinition,
) {
	for idx, portDefinition := range nodeDefinition.Ports {
		normalized := normalizePortDefinition(portDefinition)
		if normalized == portDefinition {
			continue
		}

		diagnostics.add(Diagnostic{
			Code: "host-port-pinning",
			Path: fmt.Sprintf("topology.nodes.%s.ports[%d]", nodeName, idx),
			Message: fmt.Sprintf(
				"node %q port %q host pinning was dropped; the Pod-side port %q is kept",
				nodeName,
				portDefinition,
				normalized,
			),
			Warning: true,
		})

		nodeDefinition.Ports[idx] = normalized
	}
}

// consumeExposePortsLabel translates c9s' portable containerlab label directive into the same
// destination-port intent a direct Node declares in spec.ports. A label is used at the source
// boundary because adding a normal containerlab ports entry would publish that port on the local
// Docker host. The directive is consumed here -- before reserved label filtering -- and never
// becomes Kubernetes metadata.
func consumeExposePortsLabel(
	diagnostics *compileDiagnostics,
	nodeName string,
	nodeDefinition *clabtypes.NodeDefinition,
) {
	value, ok := nodeDefinition.Labels[labelExposePorts]
	if !ok {
		return
	}

	delete(nodeDefinition.Labels, labelExposePorts)

	seenDestinations := make(map[string]bool, len(nodeDefinition.Ports))

	for _, portDefinition := range nodeDefinition.Ports {
		typedPort, err := processPortDefinition(portDefinition)
		if err != nil {
			// Existing ports are validated by the Node API/controller. Do not turn this source
			// compatibility helper into a second validator for the ordinary ports field.
			continue
		}

		seenDestinations[canonicalPortDefinition(typedPort)] = true
	}

	for idx, portDefinition := range strings.Split(value, ",") {
		portDefinition = strings.TrimSpace(portDefinition)

		typedPort, err := processPortDefinition(portDefinition)
		if err != nil {
			diagnostics.add(Diagnostic{
				Code: "invalid-expose-ports-label",
				Path: fmt.Sprintf(
					"topology.nodes.%s.labels.%s[%d]",
					nodeName,
					labelExposePorts,
					idx,
				),
				Message: fmt.Sprintf(
					"node %q expose ports label entry %q is invalid: %s",
					nodeName,
					portDefinition,
					err,
				),
			})

			continue
		}

		canonical := canonicalPortDefinition(typedPort)
		if seenDestinations[canonical] {
			continue
		}

		seenDestinations[canonical] = true
		nodeDefinition.Ports = append(nodeDefinition.Ports, canonical)
	}
}

func canonicalPortDefinition(port *typedPort) string {
	return fmt.Sprintf(
		"%d/%s",
		port.DestinationPort,
		strings.ToLower(port.Protocol),
	)
}

// consumeAppProtocolsLabel translates c9s' portable application-protocol directive into Node
// intent after containerlab label inheritance has selected its effective value. The label is
// consumed even when invalid so it can never leak into Kubernetes metadata.
func consumeAppProtocolsLabel(
	diagnostics *compileDiagnostics,
	nodeName string,
	nodeDefinition *clabtypes.NodeDefinition,
) []AppProtocol {
	value, ok := nodeDefinition.Labels[labelAppProtocols]
	if !ok {
		return nil
	}

	delete(nodeDefinition.Labels, labelAppProtocols)

	entries := strings.Split(value, ",")
	appProtocols := make([]AppProtocol, 0, len(entries))
	seenPorts := make(map[string]bool, len(entries))

	for idx, rawEntry := range entries {
		entry := strings.TrimSpace(rawEntry)
		portDefinition, appProtocol, hasSeparator := strings.Cut(entry, "=")
		if !hasSeparator {
			addInvalidAppProtocolsLabelDiagnostic(
				diagnostics,
				nodeName,
				idx,
				entry,
				"expected <port-definition>=<appProtocol>",
			)

			continue
		}

		portDefinition = strings.TrimSpace(portDefinition)
		appProtocol = strings.TrimSpace(appProtocol)

		typedPort, err := processPortDefinition(portDefinition)
		if err != nil {
			addInvalidAppProtocolsLabelDiagnostic(
				diagnostics,
				nodeName,
				idx,
				entry,
				err.Error(),
			)

			continue
		}

		canonicalPort := canonicalPortDefinition(typedPort)
		if seenPorts[canonicalPort] {
			addInvalidAppProtocolsLabelDiagnostic(
				diagnostics,
				nodeName,
				idx,
				entry,
				fmt.Sprintf("destination port %q is duplicated after normalization", canonicalPort),
			)

			continue
		}

		seenPorts[canonicalPort] = true

		if problems := k8svalidation.IsQualifiedName(appProtocol); appProtocol != "" &&
			len(problems) != 0 {
			addInvalidAppProtocolsLabelDiagnostic(
				diagnostics,
				nodeName,
				idx,
				entry,
				strings.Join(problems, "; "),
			)

			continue
		}

		appProtocols = append(appProtocols, AppProtocol{
			Port:        canonicalPort,
			AppProtocol: appProtocol,
		})
	}

	return appProtocols
}

func addInvalidAppProtocolsLabelDiagnostic(
	diagnostics *compileDiagnostics,
	nodeName string,
	entryIndex int,
	entry,
	reason string,
) {
	diagnostics.add(Diagnostic{
		Code: "invalid-app-protocols-label",
		Path: fmt.Sprintf(
			"topology.nodes.%s.labels.%s[%d]",
			nodeName,
			labelAppProtocols,
			entryIndex,
		),
		Message: fmt.Sprintf(
			"node %q application protocols label entry %q is invalid: %s",
			nodeName,
			entry,
			reason,
		),
	})
}

// dropUnusableNodeLabels records containerlab node labels that cannot be carried onto the emitted
// Node's metadata. The compile fails after collecting diagnostics, so the temporary flattened Node
// can be pruned without silently changing any emitted resource.
func dropUnusableNodeLabels(
	diagnostics *compileDiagnostics,
	nodeName string,
	nodeDefinition *clabtypes.NodeDefinition,
) {
	for key, value := range nodeDefinition.Labels {
		var reason string

		switch {
		case isReservedNodeLabel(key):
			reason = "the label is reserved by c9s"
		default:
			problems := append(
				k8svalidation.IsQualifiedName(key),
				k8svalidation.IsValidLabelValue(value)...,
			)
			if len(problems) == 0 {
				continue
			}

			reason = strings.Join(problems, "; ")
		}

		diagnostics.add(Diagnostic{
			Code: "unusable-node-label",
			Path: fmt.Sprintf("topology.nodes.%s.labels.%s", nodeName, key),
			Message: fmt.Sprintf(
				"node %q label %q cannot become a Kubernetes label: %s",
				nodeName,
				key,
				reason,
			),
		})

		delete(nodeDefinition.Labels, key)
	}
}

func isReservedNodeLabel(key string) bool {
	if strings.HasPrefix(key, labelPrefix+"/") {
		return true
	}

	switch key {
	case "app.kubernetes.io/name", labelApp, labelName,
		labelOwner, labelKind, labelNode:
		return true
	default:
		return false
	}
}

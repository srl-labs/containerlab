// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package compile renders containerlab topologies into clabernetes (c9s) primitive
// resources: Node, Link, and NodeProfile manifests.
//
// The engine is the contract between containerlab and the clabernetes repository: it takes a
// rendered containerlab topology definition plus the deployment policy the clabernetes Topology
// resource carries, and emits self contained primitive manifests that the c9s API server and
// controllers accept. The clabernetes repository consumes this package so both containerlab's
// direct c9s runtime and the c9s controller compile identically; containerlab itself imports
// nothing from clabernetes.
package compile

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	clabtypes "github.com/srl-labs/containerlab/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Logger is the minimal logging surface the compiler needs. Both containerlab's charmbracelet
// logger and clabernetes' logging instances satisfy it.
type Logger interface {
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Diagnostic describes one source construct the compile cannot faithfully preserve.
type Diagnostic struct {
	Code    string
	Path    string
	Line    int
	Message string
	// Warning marks a construct that is accepted with an adjusted or ignored meaning instead
	// of failing the compile: the source stays valid, and the diagnostic tells the author what
	// changed. Only constructs whose loss cannot silently change lab behavior may be warnings.
	Warning bool
}

// UnsupportedFieldPolicy is retained for callers that explicitly requested strict compilation.
// Error is the only supported policy; Topology compilation no longer has a warning mode.
type UnsupportedFieldPolicy string

const (
	// UnsupportedFieldPolicyError rejects every source field the compile cannot preserve.
	UnsupportedFieldPolicyError UnsupportedFieldPolicy = "error"
)

// Options are retained for strict external compiler callers.
type Options struct {
	UnsupportedFieldPolicy UnsupportedFieldPolicy
}

// UnsupportedFeaturesError reports all unsupported source constructs found in one compile pass.
// Diagnostics are sorted so CLI errors and tests remain stable across map iteration order.
type UnsupportedFeaturesError struct {
	Diagnostics []Diagnostic
}

func (e *UnsupportedFeaturesError) Error() string {
	if e == nil || len(e.Diagnostics) == 0 {
		return "topology contains features unsupported by c9s"
	}

	parts := make([]string, 0, len(e.Diagnostics))
	for _, diagnostic := range e.Diagnostics {
		parts = append(parts, formatDiagnostic(diagnostic))
	}

	return "topology contains features unsupported by c9s: " + strings.Join(parts, "; ")
}

func formatDiagnostic(diagnostic Diagnostic) string {
	location := diagnostic.Path
	if location == "" {
		location = "topology"
	}

	if diagnostic.Line > 0 {
		location = fmt.Sprintf("%s (line %d)", location, diagnostic.Line)
	}

	return fmt.Sprintf("%s: %s", location, diagnostic.Message)
}

type compileDiagnostics struct {
	diagnostics []Diagnostic
}

func newCompileDiagnostics() *compileDiagnostics {
	return &compileDiagnostics{}
}

func (d *compileDiagnostics) add(diagnostic Diagnostic) {
	d.diagnostics = append(d.diagnostics, diagnostic)
}

func (d *compileDiagnostics) warnings() []Diagnostic {
	warnings := []Diagnostic(nil)

	for _, diagnostic := range d.diagnostics {
		if diagnostic.Warning {
			warnings = append(warnings, diagnostic)
		}
	}

	return warnings
}

func (d *compileDiagnostics) err() error {
	diagnostics := []Diagnostic(nil)

	for _, diagnostic := range d.diagnostics {
		if !diagnostic.Warning {
			diagnostics = append(diagnostics, diagnostic)
		}
	}

	if len(diagnostics) == 0 {
		return nil
	}

	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}

		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}

		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}

		return diagnostics[i].Message < diagnostics[j].Message
	})

	return &UnsupportedFeaturesError{Diagnostics: diagnostics}
}

// CompiledLink holds a single wire of a compiled topology definition -- exactly the payload of
// a Link spec.
type CompiledLink struct {
	// EndpointA is the "a" side of the wire.
	EndpointA Endpoint
	// EndpointB is the "b" side of the wire.
	EndpointB Endpoint
	// MTU is the mtu of the wire (zero means unset).
	MTU int
}

// Endpoint identifies one side of a compiled wire.
type Endpoint struct {
	// NodeName is the node the endpoint sits on, or LinkHostNodeName for a host link.
	NodeName string
	// InterfaceName is the interface name on the node.
	InterfaceName string
}

// AppProtocol is the application-protocol intent for one expose Service destination port.
type AppProtocol struct {
	// Port is the destination port and transport in canonical "<number>/tcp" or "<number>/udp"
	// form.
	Port string
	// AppProtocol is a Kubernetes qualified name, or empty to suppress a built-in hint.
	AppProtocol string
}

// CompiledTopology is what a Topology definition compiles down to: flat, self contained node
// definitions (topology defaults/kinds expanded into every node), the wires between them, and
// the topology level management network settings. The compiler emits this as Node and Link
// manifests (plus NodeProfiles for deployment policy) -- all actual reconciliation
// happens in the c9s node/link controllers, identically for compiled and hand written objects.
type CompiledTopology struct {
	// Kind is the topology definition kind -- containerlab.
	Kind string
	// Nodes maps (containerlab) node name to its flattened node definition.
	Nodes map[string]*clabtypes.NodeDefinition
	// AppProtocols maps node names to c9s-specific application-protocol intent consumed from the
	// flattened containerlab labels. It stays outside Nodes so NodeDefinition remains containerlab
	// vocabulary.
	AppProtocols map[string][]AppProtocol
	// Links holds the wires of the topology.
	Links []CompiledLink
	// Mgmt holds the containerlab management network settings (if any).
	Mgmt *clabtypes.MgmtNet
	// NodeNameSources maps the name of every node that had to be renamed for Kubernetes back to
	// the name the definition uses. Node-keyed policy on the Topology (files, resources, probes)
	// is written against the definition's names, so the renderers translate through this.
	NodeNameSources map[string]string
}

// SourceNodeName returns the name the definition uses for a compiled node. It is the compiled name
// itself for every node Kubernetes could carry as written.
func (t *CompiledTopology) SourceNodeName(nodeName string) string {
	if sourceName, renamed := t.NodeNameSources[nodeName]; renamed {
		return sourceName
	}

	return nodeName
}

// errUnsupportedFieldPolicy marks a strict compiler caller requesting an unknown field policy.
var errUnsupportedFieldPolicy = errors.New("unsupported topology compiler field policy")

// TopologyKindContainerlab is the kind marker the compiler reports for containerlab definitions.
const TopologyKindContainerlab = "containerlab"

// Compile parses and compiles the containerlab topology definition carried by the input into
// primitive manifests. The definition must be self contained in containerlab vocabulary; every
// emitted manifest carries the flattened node definition, so no topology defaults or kinds
// survive into the API.
func Compile(logger Logger, input *Input) (*CompiledTopology, error) {
	if input.Definition == "" {
		return nil, fmt.Errorf("topology definition must include a containerlab topology")
	}

	return compileContainerlabDefinition(logger, input, newCompileDiagnostics())
}

// CompileWithOptions is the entry point for strict external compiler callers. An omitted policy
// and Error both select the same fail-closed compiler.
func CompileWithOptions(
	logger Logger,
	input *Input,
	options Options,
) (*CompiledTopology, error) {
	if options.UnsupportedFieldPolicy != "" &&
		options.UnsupportedFieldPolicy != UnsupportedFieldPolicyError {
		return nil, fmt.Errorf(
			"%w %q; only %q is supported",
			errUnsupportedFieldPolicy,
			options.UnsupportedFieldPolicy,
			UnsupportedFieldPolicyError,
		)
	}

	return Compile(logger, input)
}

// CompileTopology renders the compiled topology into the primitive manifests the c9s API server
// accepts: NodeProfiles first, then Nodes, then Links, every object as unstructured.
func CompileTopology(
	input *Input,
	compiled *CompiledTopology,
) (nodes, links, nodeProfiles []unstructured.Unstructured, err error) {
	return renderAll(input, compiled)
}

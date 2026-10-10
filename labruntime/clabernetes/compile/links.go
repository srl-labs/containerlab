// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

// linkDefinition represents a link definition in the topology file. It accepts both
// containerlab's brief "node:interface" endpoint syntax and the equivalent structured
// node/interface syntax used by explicit veth links.
type linkDefinition struct {
	linkConfig `yaml:",inline"`

	Type string `yaml:"type,omitempty"`
}

// linkConfig is the link config object the compiler reads.
type linkConfig struct {
	Endpoints linkEndpoints
	Labels    map[string]string `yaml:"labels,omitempty"`
	Vars      map[string]any    `yaml:"vars,omitempty"`
	MTU       int               `yaml:"mtu,omitempty"`
}

// linkEndpoints stores the canonical brief endpoint form, because that is the complete endpoint
// vocabulary the c9s Link API can represent.
type linkEndpoints []string

func (e *linkEndpoints) UnmarshalYAML(value *yamlv3.Node) error {
	if value.Kind != yamlv3.SequenceNode {
		return fmt.Errorf("%w: link endpoints must be a sequence", errParse)
	}

	endpoints := make(linkEndpoints, 0, len(value.Content))

	for _, endpointNode := range value.Content {
		switch endpointNode.Kind {
		case yamlv3.ScalarNode:
			endpoint, err := canonicalLinkEndpoint(endpointNode.Value)
			if err != nil {
				return err
			}

			endpoints = append(endpoints, endpoint)
		case yamlv3.MappingNode:
			endpoint := struct {
				Node      string `yaml:"node"`
				Interface string `yaml:"interface"`
			}{}

			err := endpointNode.Decode(&endpoint)
			if err != nil {
				return fmt.Errorf(
					"%w: decoding structured link endpoint: %w",
					errParse,
					err,
				)
			}

			err = validateStructuredLinkEndpoint(endpointNode)
			if err != nil {
				return err
			}

			canonical, err := canonicalLinkEndpoint(
				fmt.Sprintf("%s:%s", endpoint.Node, endpoint.Interface),
			)
			if err != nil {
				return err
			}

			endpoints = append(endpoints, canonical)
		case yamlv3.DocumentNode, yamlv3.SequenceNode, yamlv3.AliasNode:
			return fmt.Errorf(
				"%w: link endpoint must be a string or node/interface mapping",
				errParse,
			)
		}
	}

	*e = endpoints

	return nil
}

func canonicalLinkEndpoint(value string) (string, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != linkEndpointElementCount {
		return "", fmt.Errorf(
			"%w: link endpoint %q must use node:interface syntax",
			errParse,
			value,
		)
	}

	nodeName := strings.TrimSpace(parts[0])

	interfaceName := strings.TrimSpace(parts[1])
	if nodeName == "" || interfaceName == "" {
		return "", fmt.Errorf(
			"%w: link endpoint %q requires non-empty node and interface",
			errParse,
			value,
		)
	}

	return fmt.Sprintf("%s:%s", nodeName, interfaceName), nil
}

func validateStructuredLinkEndpoint(value *yamlv3.Node) error {
	for index := 0; index+1 < len(value.Content); index += 2 {
		key := value.Content[index]
		field := value.Content[index+1]

		if key.Value != "node" && key.Value != "interface" {
			continue
		}

		if field.Kind != yamlv3.ScalarNode || field.Tag != "!!str" {
			return fmt.Errorf(
				"%w: structured link endpoint field %q must be a string",
				errParse,
				key.Value,
			)
		}
	}

	return nil
}

// compileContainerlabLinks converts the containerlab links section into compiled wires.
func compileContainerlabLinks( //nolint:gocyclo
	fileTopology *fileTopology,
	diagnostics *compileDiagnostics,
) ([]CompiledLink, error) {
	links := make([]CompiledLink, 0, len(fileTopology.Links))

	for linkIndex, link := range fileTopology.Links {
		linkPath := fmt.Sprintf("topology.links[%d]", linkIndex)
		if len(link.Labels) != 0 {
			diagnostics.add(Diagnostic{
				Code:    "unsupported-link-labels",
				Path:    linkPath + ".labels",
				Message: "link labels are not preserved by the c9s Link API",
			})
		}

		if len(link.Vars) != 0 {
			diagnostics.add(Diagnostic{
				Code:    "unsupported-link-vars",
				Path:    linkPath + ".vars",
				Message: "link vars are not preserved by the c9s Link API",
			})
		}

		switch link.Type {
		case "", "brief", "veth":
		default:
			diagnostics.add(unsupportedContainerlabLinkTypeDiagnostic(link.Type, linkPath))

			continue
		}

		if len(link.Endpoints) != linkEndpointElementCount {
			return nil, fmt.Errorf(
				"%w: endpoint '%q' has wrong syntax, unexpected number of items",
				errParse,
				[]string(link.Endpoints),
			)
		}

		endpointAParts := strings.Split(link.Endpoints[0], ":")
		endpointBParts := strings.Split(link.Endpoints[1], ":")

		if len(endpointAParts) != linkEndpointElementCount ||
			len(endpointBParts) != linkEndpointElementCount {
			return nil, fmt.Errorf(
				"%w: endpoint '%q' has wrong syntax, bad node:interface config",
				errParse,
				[]string(link.Endpoints),
			)
		}

		invalidEndpoint := false

		for endpointIndex, endpointParts := range [][]string{endpointAParts, endpointBParts} {
			nodeName := endpointParts[0]

			if nodeName == LinkHostNodeName {
				continue
			}

			if nodeName == "mgmt-net" || nodeName == "macvlan" {
				diagnostics.add(Diagnostic{
					Code: "unsupported-special-endpoint",
					Path: fmt.Sprintf("%s.endpoints[%d]", linkPath, endpointIndex),
					Message: fmt.Sprintf(
						"special endpoint %q requires host networking that c9s does not provide",
						nodeName,
					),
				})

				invalidEndpoint = true

				continue
			}

			if _, exists := fileTopology.Nodes[nodeName]; !exists {
				diagnostics.add(Diagnostic{
					Code:    "unknown-link-endpoint",
					Path:    fmt.Sprintf("%s.endpoints[%d]", linkPath, endpointIndex),
					Message: fmt.Sprintf("link endpoint references nonexistent node %q", nodeName),
				})

				invalidEndpoint = true
			}
		}

		if endpointAParts[0] == LinkHostNodeName &&
			endpointBParts[0] == LinkHostNodeName {
			diagnostics.add(Diagnostic{
				Code:    "invalid-host-link",
				Path:    linkPath + ".endpoints",
				Message: "a c9s host link must have exactly one Node endpoint",
			})

			invalidEndpoint = true
		}

		if invalidEndpoint {
			continue
		}

		links = append(links, CompiledLink{
			EndpointA: Endpoint{
				NodeName:      endpointAParts[0],
				InterfaceName: endpointAParts[1],
			},
			EndpointB: Endpoint{
				NodeName:      endpointBParts[0],
				InterfaceName: endpointBParts[1],
			},
			MTU: link.MTU,
		})
	}

	return links, nil
}

func unsupportedContainerlabLinkTypeDiagnostic(linkType, linkPath string) Diagnostic {
	message := fmt.Sprintf(
		"native link type %q has no c9s topology-link equivalent",
		linkType,
	)

	if linkType == "host" {
		message = fmt.Sprintf(
			"explicit native link type %q requires structured endpoints that the "+
				"c9s topology compiler does not support; use brief endpoints",
			linkType,
		)
	}

	return Diagnostic{
		Code:    "unsupported-link-type",
		Path:    linkPath + ".type",
		Message: message,
	}
}

/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
	"gopkg.in/yaml.v2"
)

var interfaceFormat = map[string]string{
	"srl":  "e1-%d",
	"ceos": "eth%d",
}

const (
	defaultSRLType     = "ixr6"
	defaultNodePrefix  = "node"
	defaultGroupPrefix = "group"
)

var image string
var kind string
var nodes []string
var license []string
var nodePrefix string
var groupPrefix string
var file string

type nodesDef struct {
	numNodes uint
	kind     string
	typ      string
}

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "generate a Clos topology file, based on provided flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return errors.New("--name is mandatory")
		}
		nodeDefs, licenses, err := parseInput(kind, license, nodes...)
		if err != nil {
			return err
		}
		b, err := generateTopologyConfig(name, mgmtNetName, mgmtIPv4Subnet.String(), mgmtIPv6Subnet.String(), image, licenses, nodeDefs...)
		if err != nil {
			return err
		}
		if file == "" {
			fmt.Println(string(b))
			return nil
		}
		file, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(b)
		return err
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&mgmtNetName, "network", "", "clab", "management network name")
	generateCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	generateCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	generateCmd.Flags().StringVarP(&image, "image", "", "", "container image name")
	generateCmd.Flags().StringVarP(&kind, "kind", "", "srl", "container kind")
	generateCmd.Flags().StringSliceVarP(&nodes, "nodes", "", []string{}, "comma separated nodes definitions in format <num_nodes>:<kind>:<type>, each defining a Clos network stage")
	generateCmd.Flags().StringSliceVarP(&license, "license", "", []string{}, "path to license file, can be prefix with the node kind. <kind>:/path/to/file")
	generateCmd.Flags().StringVarP(&nodePrefix, "node-prefix", "", defaultNodePrefix, "prefix used in node names")
	generateCmd.Flags().StringVarP(&groupPrefix, "group-prefix", "", defaultGroupPrefix, "prefix used in group names")
	generateCmd.Flags().StringVarP(&file, "file", "", "", "file path to save generated topology")
}

func generateTopologyConfig(name, network, ipv4range, ipv6range, image string, licenses map[string]string, nodes ...nodesDef) ([]byte, error) {
	numStages := len(nodes)
	config := &clab.Config{
		Name: name,
		Topology: clab.Topology{
			Kinds: make(map[string]clab.NodeConfig),
			Nodes: make(map[string]clab.NodeConfig),
			Defaults: clab.NodeConfig{
				Image: image,
				//License: license,
			},
		},
	}
	config.Mgmt.Network = network
	if ipv4range != "<nil>" {
		config.Mgmt.Ipv4Subnet = ipv4range
	}
	if ipv6range != "<nil>" {
		config.Mgmt.Ipv6Subnet = ipv6range
	}
	for k, lic := range licenses {
		config.Topology.Kinds[k] = clab.NodeConfig{License: lic}
	}
	for i := 0; i < numStages-1; i++ {
		interfaceOffset := uint(0)
		if i > 0 {
			interfaceOffset = nodes[i-1].numNodes
		}
		for j := uint(0); j < nodes[i].numNodes; j++ {
			node1 := fmt.Sprintf("%s%d-%d", nodePrefix, i+1, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = clab.NodeConfig{
					Group: fmt.Sprintf("%s-%d", groupPrefix, i+1),
					Kind:  nodes[i].kind,
					Type:  nodes[i].typ,
				}
			}
			for k := uint(0); k < nodes[i+1].numNodes; k++ {
				node2 := fmt.Sprintf("%s%d-%d", nodePrefix, i+2, k+1)
				if _, ok := config.Topology.Nodes[node2]; !ok {
					config.Topology.Nodes[node2] = clab.NodeConfig{
						Group: fmt.Sprintf("%s-%d", groupPrefix, i+2),
						Kind:  nodes[i+1].kind,
						Type:  nodes[i+1].typ,
					}
				}
				config.Topology.Links = append(config.Topology.Links, clab.LinkConfig{
					Endpoints: []string{
						node1 + ":" + fmt.Sprintf(interfaceFormat[nodes[i].kind], k+1+interfaceOffset),
						node2 + ":" + fmt.Sprintf(interfaceFormat[nodes[i+1].kind], j+1),
					},
				})
			}
		}
	}
	return yaml.Marshal(config)
}
func parseInput(kind string, license []string, nodes ...string) ([]nodesDef, map[string]string, error) {
	licenses, err := parseLicenseFlag(kind, license)
	if err != nil {
		return nil, nil, err
	}
	result, err := parseNodes(kind, nodes...)
	return result, licenses, err
}

func parseLicenseFlag(kind string, license []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, lic := range license {
		items := strings.SplitN(lic, ":", 2)
		switch len(items) {
		case 1:
			if _, ok := result[kind]; !ok {
				result[kind] = items[0]
			} else {
				return nil, fmt.Errorf("duplicated license for kind '%s'", kind)
			}
		case 2:
			if _, ok := result[items[0]]; !ok {
				result[items[0]] = items[1]
			} else {
				return nil, fmt.Errorf("duplicated license for kind '%s'", items[1])
			}
		}
	}
	return result, nil
}

func parseNodes(kind string, nodes ...string) ([]nodesDef, error) {
	numStages := len(nodes)
	if numStages == 0 {
		return nil, errors.New("no nodes specified using --nodes")
	}
	result := make([]nodesDef, numStages)
	for idx, n := range nodes {
		def := nodesDef{}
		items := strings.SplitN(n, ":", 3)
		if len(items) == 0 {
			return nil, fmt.Errorf("wrong --nodes format '%s'", n)
		}
		i, err := strconv.Atoi(items[0])
		if err != nil {
			return nil, fmt.Errorf("failed converting '%s' to an integer: %v", items[0], err)
		}
		def.numNodes = uint(i)
		switch len(items) {
		case 1:
			def.kind = kind
			if kind == "srl" {
				def.typ = defaultSRLType
			}
		case 2:
			switch items[1] {
			case "ceos":
				def.kind = items[1]
			case "srl":
				def.kind = items[1]
				def.typ = defaultSRLType
			default:
				def.kind = kind
				def.typ = items[1]
			}
		case 3:
			def.numNodes = uint(i)
			def.kind = items[1]
			def.typ = items[2]
		}
		result[idx] = def
	}
	return result, nil
}

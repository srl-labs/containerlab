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
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
	"gopkg.in/yaml.v2"
)

var interfaceFormat = map[string]string{
	"srl": "e1-%d",
}

var image string
var kind string
var nodes []uint
var license string

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:     "generate",
	Aliases: []string{"gen"},
	Short:   "generate a Clos topology file, based on provided flags",
	RunE: func(cmd *cobra.Command, args []string) error {
		generateTopologyConfig(name, mgmtNetName, mgmtIPv4Subnet.String(), mgmtIPv6Subnet.String(), image, kind, nodes...)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&mgmtNetName, "network", "", "", "management network name")
	generateCmd.Flags().IPNetVarP(&mgmtIPv4Subnet, "ipv4-subnet", "4", net.IPNet{}, "management network IPv4 subnet range")
	generateCmd.Flags().IPNetVarP(&mgmtIPv6Subnet, "ipv6-subnet", "6", net.IPNet{}, "management network IPv6 subnet range")
	generateCmd.Flags().StringVarP(&image, "image", "", "", "container image name")
	generateCmd.Flags().StringVarP(&kind, "kind", "", "srl", "container kind")
	generateCmd.Flags().UintSliceVarP(&nodes, "nodes", "", []uint{}, "comma separated integers represeting the number of nodes per Clos network stage")
	generateCmd.Flags().StringVarP(&license, "license", "", "", "path to license file")
}

func generateTopologyConfig(name, network, ipv4range, ipv6range, image, kind string, nodes ...uint) *clab.Config {
	config := &clab.Config{
		Name: name,
		Topology: clab.Topology{
			Nodes: make(map[string]clab.NodeConfig),
			Defaults: clab.NodeConfig{
				Kind:    kind,
				Image:   image,
				
			},
		},
	}
	config.Mgmt.Network = network
	config.Mgmt.Ipv4Subnet = ipv4range
	config.Mgmt.Ipv6Subnet = ipv6range

	numStages := len(nodes)
	for i := 0; i < numStages-1; i++ {
		interfaceOffset := uint(0)
		if i > 0 {
			interfaceOffset = nodes[i-1]
		}
		for j := uint(0); j < nodes[i]; j++ {
			node1 := fmt.Sprintf("node%d-%d", i+1, j+1)
			if _, ok := config.Topology.Nodes[node1]; !ok {
				config.Topology.Nodes[node1] = clab.NodeConfig{
					Group:   fmt.Sprintf("group-%d", i+1),
					Type:    "ixr6",
					License: license,
				}
			}
			for k := uint(0); k < nodes[i+1]; k++ {
				node2 := fmt.Sprintf("node%d-%d", i+2, k+1)
				if _, ok := config.Topology.Nodes[node2]; !ok {
					config.Topology.Nodes[node2] = clab.NodeConfig{
						Group:   fmt.Sprintf("group-%d", i+2),
						Type:    "ixr6",
						License: license,
					}
				}
				config.Topology.Links = append(config.Topology.Links, clab.LinkConfig{
					Endpoints: []string{
						node1 + ":" + fmt.Sprintf(interfaceFormat[kind], k+1+interfaceOffset),
						node2 + ":" + fmt.Sprintf(interfaceFormat[kind], j+1),
					},
				})
			}
		}
	}

	out, err := yaml.Marshal(config)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(out))
	return config
}

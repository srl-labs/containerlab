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

	"github.com/spf13/cobra"
	"github.com/srl-wim/container-lab/clab"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "generate a topology file based on provided flags",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate called")
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

func generateTopologyConfig(name, network, ipv4range, ipv6range, image string, nodes ...uint) *clab.Config {
	config := &clab.Config{
		Name: name,
	}
	config.Mgmt.Network = network
	config.Mgmt.Ipv4Subnet = ipv4range
	config.Mgmt.Ipv6Subnet = ipv6range
	numStages := len(nodes)
	for i := 0; i < numStages-1; i += 2 {
		for j := uint(0); j < nodes[i]; j++ {
			for k := uint(0); k < nodes[i+1]; k++ {
				config.Topology.Nodes[fmt.Sprintf("node%d-%d", j, k)] = clab.NodeConfig{
					Group: fmt.Sprintf("group-%d", j),
					Image: image,
				}
				config.Topology.Links = append(config.Topology.Links, clab.Link{
					A: &clab.Endpoint{},
					B: &clab.Endpoint{},
				})
			}
		}
	}
	return nil
}

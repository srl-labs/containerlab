package main

// import (
// 	"fmt"

// 	log "github.com/sirupsen/logrus"
// 	"github.com/srl-labs/containerlab/links"
// 	"gopkg.in/yaml.v2"
// )

// var yamlData = `
// nodes:
//     bla: foo
//     blubb: peng
// links:
//     - endpoints: ["srl:eth1", "srl2:eth3"]
//     - type: veth
//       mtu: 1500
//       endpoints:
//       - node:          srl1
//         interface:     ethernet-1/1
//       - node:        srl2
//         interface:    ethernet-1/1
//     - type: host
//       host-interface:    srl1_e1-2
//       node:             srl1
//       node-interface:    ethernet-1/2
//       labels:
//         foo: bar
//     - type: macvlan
//       host-interface:    eno0
//       node:             srl1
//       node-interface:    ethernet-1/3
//     - type: macvtap
//       host-interface:    eno0
//       node:             srl1
//       node-interface:    ethernet-1/4
//     - type: mgmt-net
//       host-interface:    srl1_e1-5
//       node:             srl1
//       node-interface:    ethernet-1/5
// `

// type ClabConfig struct {
// 	Nodes map[string]interface{} `yaml:"nodes"`
// 	Links []*links.RawLinkType   `yaml:"links"`
// }

// func main() {
// 	var c ClabConfig
// 	err := yaml.Unmarshal([]byte(yamlData), &c)
// 	if err != nil {
// 		log.Error(err)
// 	}
// 	fmt.Println("Done")
// }

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	_ "embed"
	"io"
	"os"
	"sort"
	"text/template"

	"github.com/srl-labs/containerlab/types"
)

//go:embed inventory_ansible.go.tpl
var ansibleInvT string

// AnsibleInventoryNode represents the data structure used to generate the ansible inventory file.
// It embeds the NodeConfig struct and adds the Username and Password fields extracted from
// the node registry.
type AnsibleInventoryNode struct {
	*types.NodeConfig

	Username string
	Password string

	NetworkOS   string
	AnsibleConn string
}

// AnsibleInventory represents the data structure used to generate the ansible inventory file.
type AnsibleInventory struct {
	// clab nodes aggregated by their kind
	Nodes map[string][]*AnsibleInventoryNode
	// clab nodes aggregated by user-defined groups
	Groups map[string][]*AnsibleInventoryNode
}

// GenerateInventories generate various inventory files and writes it to a lab location.
func (c *CLab) GenerateInventories() error {
	ansibleInvFPath := c.TopoPaths.AnsibleInventoryFileAbsPath()
	f, err := os.Create(ansibleInvFPath)
	if err != nil {
		return err
	}

	return c.generateAnsibleInventory(f)
}

// generateAnsibleInventory generates and writes ansible inventory file to w.
func (c *CLab) generateAnsibleInventory(w io.Writer) error {

	i := AnsibleInventory{
		Nodes:  make(map[string][]*AnsibleInventoryNode),
		Groups: make(map[string][]*AnsibleInventoryNode),
	}

	for _, n := range c.Nodes {
		ansibleNode := &AnsibleInventoryNode{
			NodeConfig: n.Config(),
		}

		// add username and password to the node
		nodeRegEntry := c.Reg.Kind(n.Config().Kind)
		if nodeRegEntry != nil {
			ansibleNode.Username = nodeRegEntry.Credentials().GetUsername()
			ansibleNode.Password = nodeRegEntry.Credentials().GetPassword()
		}

		// add network_os to the node
		ansibleNode.setNetworkOS()
		// add ansible_connection to the node
		ansibleNode.setAnsibleConnection()

		i.Nodes[n.Config().Kind] = append(i.Nodes[n.Config().Kind], ansibleNode)
		if n.Config().Labels["ansible-group"] != "" {
			i.Groups[n.Config().Labels["ansible-group"]] =
				append(i.Groups[n.Config().Labels["ansible-group"]], ansibleNode)
		}
	}

	// sort nodes by name as they are not sorted originally
	for _, nodes := range i.Nodes {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	// sort nodes-per-group by name as they are not sorted originally
	for _, nodes := range i.Groups {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	t, err := template.New("ansible").Parse(ansibleInvT)
	if err != nil {
		return err
	}
	err = t.Execute(w, i)
	if err != nil {
		return err
	}

	return err
}

// setNetworkOS sets the network_os variable for the kind.
func (n *AnsibleInventoryNode) setNetworkOS() {
	switch n.Kind {
	case "nokia_srlinux", "srl":
		n.NetworkOS = "nokia.srlinux.srlinux"
	case "nokia_sros", "vr-sros":
		n.NetworkOS = "nokia.sros.md"
	}
}

// setAnsibleConnection sets the ansible_connection variable for the kind.
func (n *AnsibleInventoryNode) setAnsibleConnection() {
	switch n.Kind {
	case "nokia_srlinux", "srl":
		n.AnsibleConn = "ansible.netcommon.httpapi"
	case "nokia_sros", "vr-sros":
		n.AnsibleConn = "ansible.netcommon.network_cli"
	}
}

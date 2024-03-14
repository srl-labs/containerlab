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

	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/types"
)

//go:embed inventory_ansible.go.tpl
var ansibleInvT string

// AnsibleInventoryNode represents the data structure used to generate the ansible inventory file.
// It embeds the NodeConfig struct and adds the Username and Password fields extracted from
// the node registry.
type AnsibleInventoryNode struct {
	*types.NodeConfig
}

// KindProps is the kind properties structure used to generate the ansible inventory file.
type KindProps struct {
	Username    string
	Password    string
	NetworkOS   string
	AnsibleConn string
}

// AnsibleInventory represents the data structure used to generate the ansible inventory file.
type AnsibleInventory struct {
	// clab node kinds
	Kinds map[string]*KindProps
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
	inv := AnsibleInventory{
		Kinds:  make(map[string]*KindProps),
		Nodes:  make(map[string][]*AnsibleInventoryNode),
		Groups: make(map[string][]*AnsibleInventoryNode),
	}

	for _, n := range c.Nodes {
		ansibleNode := &AnsibleInventoryNode{
			NodeConfig: n.Config(),
		}

		// add kindprops to the inventory struct
		// the kindProps is passed as a ref and is populated
		// down below
		kindProps := &KindProps{}
		inv.Kinds[n.Config().Kind] = kindProps

		// add username and password to kind properties
		// assumption is that all nodes of the same kind have the same credentials
		nodeRegEntry := kind_registry.KindRegistryInstance.Kind(n.Config().Kind)
		if nodeRegEntry != nil {
			kindProps.Username = nodeRegEntry.Credentials().GetUsername()
			kindProps.Password = nodeRegEntry.Credentials().GetPassword()
		}

		// add network_os to the node
		kindProps.setNetworkOS(n.Config().Kind)
		// add ansible_connection to the node
		kindProps.setAnsibleConnection(n.Config().Kind)

		inv.Nodes[n.Config().Kind] = append(inv.Nodes[n.Config().Kind], ansibleNode)

		if n.Config().Labels["ansible-group"] != "" {
			inv.Groups[n.Config().Labels["ansible-group"]] =
				append(inv.Groups[n.Config().Labels["ansible-group"]], ansibleNode)
		}
	}

	// sort nodes by name as they are not sorted originally
	for _, nodes := range inv.Nodes {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	// sort nodes-per-group by name as they are not sorted originally
	for _, nodes := range inv.Groups {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	t, err := template.New("ansible").Parse(ansibleInvT)
	if err != nil {
		return err
	}
	err = t.Execute(w, inv)
	if err != nil {
		return err
	}

	return err
}

// setNetworkOS sets the network_os variable for the kind.
func (n *KindProps) setNetworkOS(kind string) {
	switch kind {
	case "nokia_srlinux", "srl":
		n.NetworkOS = "nokia.srlinux.srlinux"
	case "nokia_sros", "vr-sros":
		n.NetworkOS = "nokia.sros.md"
	}
}

// setAnsibleConnection sets the ansible_connection variable for the kind.
func (n *KindProps) setAnsibleConnection(kind string) {
	switch kind {
	case "nokia_srlinux", "srl":
		n.AnsibleConn = "ansible.netcommon.httpapi"
	case "nokia_sros", "vr-sros":
		n.AnsibleConn = "ansible.netcommon.network_cli"
	}
}

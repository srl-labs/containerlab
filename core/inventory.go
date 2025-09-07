// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	_ "embed"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"text/template"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
)

//go:embed assets/inventory_ansible.go.tpl
var ansibleInvT string

// AnsibleInventoryNode represents the data structure used to generate the ansible inventory file.
// It embeds the NodeConfig struct and adds the Username and Password fields extracted from
// the node registry.
type AnsibleInventoryNode struct {
	*clabtypes.NodeConfig
}

// AnsibleKindProps is the kind properties structure used to generate the ansible inventory file.
type AnsibleKindProps struct {
	Username    string
	Password    string
	NetworkOS   string
	AnsibleConn string
}

// AnsibleInventory represents the data structure used to generate the ansible inventory file.
type AnsibleInventory struct {
	// clab node kinds
	Kinds map[string]*AnsibleKindProps
	// clab nodes aggregated by their kind
	Nodes map[string][]*AnsibleInventoryNode
	// clab nodes aggregated by user-defined groups
	Groups map[string][]*AnsibleInventoryNode
}

// GenerateInventories generate various inventory files and writes it to a lab location.
func (c *CLab) GenerateInventories() error {
	// generate Ansible Inventory
	ansibleInvFPath := c.TopoPaths.AnsibleInventoryFileAbsPath()

	ansibleFile, err := os.Create(ansibleInvFPath)
	if err != nil {
		return err
	}

	err = c.generateAnsibleInventory(ansibleFile)
	if err != nil {
		return err
	}

	err = ansibleFile.Close()
	if err != nil {
		return err
	}

	// generate Nornir Simple Inventory
	nornirSimpleInvFPath := c.TopoPaths.NornirSimpleInventoryFileAbsPath()

	nornirFile, err := os.Create(nornirSimpleInvFPath)
	if err != nil {
		return err
	}

	err = c.generateNornirSimpleInventory(nornirFile)
	if err != nil {
		return err
	}

	return nornirFile.Close()
}

// generateAnsibleInventory generates and writes ansible inventory file to w.
func (c *CLab) generateAnsibleInventory(w io.Writer) error {
	inv := AnsibleInventory{
		Kinds:  make(map[string]*AnsibleKindProps),
		Nodes:  make(map[string][]*AnsibleInventoryNode),
		Groups: make(map[string][]*AnsibleInventoryNode),
	}

	for _, n := range c.Nodes {
		ansibleNode := &AnsibleInventoryNode{
			NodeConfig: n.Config(),
		}

		// add AnsibleKindProps to the inventory struct
		// the ansibleKindProps is passed as a ref and is populated
		// down below
		ansibleKindProps := &AnsibleKindProps{}
		inv.Kinds[n.Config().Kind] = ansibleKindProps

		// add username and password to kind properties
		// assumption is that all nodes of the same kind have the same credentials
		nodeRegEntry := c.Reg.Kind(n.Config().Kind)
		if nodeRegEntry != nil {
			ansibleKindProps.Username = nodeRegEntry.GetCredentials().GetUsername()
			ansibleKindProps.Password = nodeRegEntry.GetCredentials().GetPassword()
		}

		// add network_os to the node
		ansibleKindProps.setNetworkOS(n.Config().Kind)
		// add ansible_connection to the node
		ansibleKindProps.setAnsibleConnection(n.Config().Kind)

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
func (n *AnsibleKindProps) setNetworkOS(kind string) {
	switch kind {
	case "nokia_srlinux", "srl":
		n.NetworkOS = "nokia.srlinux.srlinux"
	case "nokia_sros", "vr-sros", "nokia_srsim":
		n.NetworkOS = "nokia.sros.md"
	}
}

// setAnsibleConnection sets the ansible_connection variable for the kind.
func (n *AnsibleKindProps) setAnsibleConnection(kind string) {
	switch kind {
	case "nokia_srlinux", "srl":
		n.AnsibleConn = "ansible.netcommon.httpapi"
	case "nokia_sros", "vr-sros", "nokia_srsim":
		n.AnsibleConn = "ansible.netcommon.network_cli"
	}
}

// Nornir Simple Inventory
// https://nornir.readthedocs.io/en/latest/tutorial/inventory.html

//go:embed assets/inventory_nornir_simple.go.tpl
var nornirSimpleInvT string

// NornirSimpleInventoryKindProps is the kind properties structure used to generate the nornir
// simple inventory file.
type NornirSimpleInventoryKindProps struct {
	Username string
	Password string
	Platform string
}

// NornirSimpleInventoryNode represents the data structure used to generate the nornir simple
// inventory file. It embeds the NodeConfig struct and adds the Username and Password fields
// extracted from the node registry.
type NornirSimpleInventoryNode struct {
	*clabtypes.NodeConfig
	NornirGroups []string
}

// NornirSimpleInventory represents the data structure used to generate the nornir simple inventory
// file.
type NornirSimpleInventory struct {
	// clab node kinds
	Kinds map[string]*NornirSimpleInventoryKindProps
	// clab nodes aggregated by their kind
	Nodes map[string][]*NornirSimpleInventoryNode
	// clab nodes aggregated by user-defined groups
	Groups map[string][]*NornirSimpleInventoryNode
}

// generateNornirSimpleInventory generates and writes a Nornir Simple inventory file to w.
func (c *CLab) generateNornirSimpleInventory(w io.Writer) error {
	inv := NornirSimpleInventory{
		Kinds:  make(map[string]*NornirSimpleInventoryKindProps),
		Nodes:  make(map[string][]*NornirSimpleInventoryNode),
		Groups: make(map[string][]*NornirSimpleInventoryNode),
	}

	platformNameSchema := os.Getenv(clabconstants.ClabEnvNornirPlatformNameSchema)

	for _, n := range c.Nodes {
		nornirNode := &NornirSimpleInventoryNode{
			NodeConfig: n.Config(),
		}

		// add nornirSimpleInventoryKindProps to the inventory struct
		// the nornirSimpleInventoryKindProps is passed as a ref and is populated
		// down below
		nornirSimpleInventoryKindProps := &NornirSimpleInventoryKindProps{}
		inv.Kinds[n.Config().Kind] = nornirSimpleInventoryKindProps

		// the nornir platform is set by default to the node's kind
		// and is overwritten with the proper Nornir Inventory Platform
		// based on the value of CLAB_PLATFORM_NAME_SCHEMA (nornir or scrapi).
		// defaults to Nornir-Napalm/Netmiko compatible platform name
		nornirSimpleInventoryKindProps.Platform = n.Config().Kind

		// add username and password to kind properties
		// assumption is that all nodes of the same kind have the same credentials
		nodeRegEntry := c.Reg.Kind(n.Config().Kind)
		if nodeRegEntry != nil {
			nornirSimpleInventoryKindProps.Username =
				nodeRegEntry.GetCredentials().GetUsername()
			nornirSimpleInventoryKindProps.Password =
				nodeRegEntry.GetCredentials().GetPassword()

			if nodeRegEntry.PlatformAttrs() != nil {
				switch platformNameSchema {
				case "napalm":
					nornirSimpleInventoryKindProps.Platform =
						nodeRegEntry.PlatformAttrs().NapalmPlatformName
				case "scrapi":
					nornirSimpleInventoryKindProps.Platform =
						nodeRegEntry.PlatformAttrs().ScrapliPlatformName
				}
			}
		}

		for key, value := range n.Config().Labels {
			if strings.HasPrefix(key, "nornir-group") {
				nornirNode.NornirGroups = append(nornirNode.NornirGroups, value)
			}
		}

		// sort by group name so it's deterministic
		slices.Sort(nornirNode.NornirGroups)

		inv.Nodes[n.Config().Kind] = append(inv.Nodes[n.Config().Kind], nornirNode)
	}

	// sort nodes by name as they are not sorted originally
	for _, nodes := range inv.Nodes {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	t, err := template.New("nornir_simple").Parse(nornirSimpleInvT)
	if err != nil {
		return err
	}

	err = t.Execute(w, inv)
	if err != nil {
		return err
	}

	return err
}

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

// AnsibleInventoryNode is used to generate the ansible inventory file.
type AnsibleInventoryNode struct {
	*clabtypes.NodeConfig
	// EmitAnsibleUserOnHost is true when ansible_user must be set on the host entry
	// (topology node/group source, or heterogeneous usernames within the Ansible group).
	EmitAnsibleUserOnHost bool
	// EmitAnsiblePasswordOnHost is true when ansible_password must be set on the host entry.
	EmitAnsiblePasswordOnHost bool
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
	// DefaultsUsername and DefaultsPassword emit under all.vars when set in topology.defaults.
	DefaultsUsername string
	DefaultsPassword string
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
	topo := c.Config.Topology
	defs := topo.GetDefaults()
	inv := AnsibleInventory{
		Kinds:            make(map[string]*AnsibleKindProps),
		Nodes:            make(map[string][]*AnsibleInventoryNode),
		Groups:           make(map[string][]*AnsibleInventoryNode),
		DefaultsUsername: defs.Credentials.Username,
		DefaultsPassword: defs.Credentials.Password,
	}

	for _, n := range c.Nodes {
		cfg := n.Config()
		ansibleGroup := ansibleInventoryGroup(cfg)

		credSrc := topo.GetNodeCredentialsTopologySource(cfg.ShortName)
		emitCredsOnHost := credSrc == clabtypes.CredentialTopologyNode ||
			credSrc == clabtypes.CredentialTopologyGroup

		ansibleNode := &AnsibleInventoryNode{
			NodeConfig:                cfg,
			EmitAnsibleUserOnHost:     emitCredsOnHost,
			EmitAnsiblePasswordOnHost: emitCredsOnHost,
		}

		inv.Nodes[ansibleGroup] = append(inv.Nodes[ansibleGroup], ansibleNode)

		if cfg.Labels["ansible-group"] != "" {
			inv.Groups[cfg.Labels["ansible-group"]] =
				append(inv.Groups[cfg.Labels["ansible-group"]], ansibleNode)
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

	for ansibleGroup, nodes := range inv.Nodes {
		c.applyAnsibleHostEmitFlagsForHeterogeneousCredentials(nodes, topo)
		inv.Kinds[ansibleGroup] = c.buildAnsibleKindProps(ansibleGroup, nodes, topo)
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

func ansibleInventoryGroup(cfg *clabtypes.NodeConfig) string {
	ansibleGroup := cfg.Kind
	if strings.EqualFold(cfg.Env["CLAB_SROS_CONFIG_MODE"], "classic") {
		ansibleGroup = "nokia_srsim_classic"
	}
	return ansibleGroup
}

func fillAnsibleConnectionFields(ansibleGroup string, props *AnsibleKindProps) {
	switch ansibleGroup {
	case "nokia_srlinux", "srl":
		props.NetworkOS = "nokia.srlinux.srlinux"
		props.AnsibleConn = "ansible.netcommon.httpapi"
	case "nokia_sros", "vr-sros", "nokia_srsim":
		props.NetworkOS = "nokia.sros.md"
		props.AnsibleConn = "ansible.netcommon.network_cli"
	case "nokia_srsim_classic":
		props.NetworkOS = "nokia.sros.classic"
		props.AnsibleConn = "ansible.netcommon.network_cli"
	}
}

func (c *CLab) applyAnsibleHostEmitFlagsForHeterogeneousCredentials(
	nodes []*AnsibleInventoryNode,
	topo *clabtypes.Topology,
) {
	if len(nodes) < 2 {
		return
	}

	u0 := nodes[0].Credentials.Username
	p0 := nodes[0].Credentials.Password
	uniformUser := true
	uniformPass := true
	for _, n := range nodes[1:] {
		if n.Credentials.Username != u0 {
			uniformUser = false
		}
		if n.Credentials.Password != p0 {
			uniformPass = false
		}
	}

	if !uniformUser {
		for _, n := range nodes {
			n.EmitAnsibleUserOnHost = true
		}
	}
	if !uniformPass {
		for _, n := range nodes {
			n.EmitAnsiblePasswordOnHost = true
		}
	}

	// Mixed topology credential sources across hosts (e.g. some from defaults, some from kind)
	// require per-host ansible_user and ansible_password even when resolved strings match.
	anyDef := false
	anyNonDef := false
	for _, n := range nodes {
		s := topo.GetNodeCredentialsTopologySource(n.ShortName)
		if s == clabtypes.CredentialTopologyDefaults {
			anyDef = true
		}
		if s != clabtypes.CredentialTopologyUnset &&
			s != clabtypes.CredentialTopologyDefaults {
			anyNonDef = true
		}
	}
	if anyDef && anyNonDef {
		for _, n := range nodes {
			n.EmitAnsibleUserOnHost = true
			n.EmitAnsiblePasswordOnHost = true
		}
	}
}

func (c *CLab) buildAnsibleKindProps(
	ansibleGroup string,
	nodes []*AnsibleInventoryNode,
	topo *clabtypes.Topology,
) *AnsibleKindProps {
	props := &AnsibleKindProps{}
	if len(nodes) == 0 {
		return props
	}

	fillAnsibleConnectionFields(ansibleGroup, props)

	first := nodes[0].NodeConfig
	kindDef := topo.GetKind(strings.ToLower(first.Kind))
	kindTopoUser := kindDef.Credentials.Username
	kindTopoPass := kindDef.Credentials.Password

	u0 := nodes[0].Credentials.Username
	p0 := nodes[0].Credentials.Password
	uniformUser := true
	uniformPass := true
	for _, n := range nodes[1:] {
		if n.Credentials.Username != u0 {
			uniformUser = false
		}
		if n.Credentials.Password != p0 {
			uniformPass = false
		}
	}

	anyFromDefaults := false
	for _, n := range nodes {
		if topo.GetNodeCredentialsTopologySource(
			n.ShortName,
		) == clabtypes.CredentialTopologyDefaults {
			anyFromDefaults = true
			break
		}
	}

	if kindTopoUser != "" {
		props.Username = kindTopoUser
	} else if uniformUser && u0 != "" && !anyFromDefaults {
		props.Username = u0
	}

	if kindTopoPass != "" {
		props.Password = kindTopoPass
	} else if uniformPass && p0 != "" && !anyFromDefaults {
		props.Password = p0
	}

	return props
}

//go:embed assets/inventory_nornir_simple.go.tpl
var nornirSimpleInvT string

// NornirSimpleInventoryKindProps is the kind properties structure used to generate the nornir
// simple inventory file.
type NornirSimpleInventoryKindProps struct {
	Username string
	Password string
	Platform string
}

// NornirSimpleInventoryNode is used to generate the nornir simple inventory file.
type NornirSimpleInventoryNode struct {
	*clabtypes.NodeConfig
	NornirGroups []string
	// EmitUsernameOnHost / EmitPasswordOnHost select per-host credential lines in the template.
	EmitUsernameOnHost bool
	EmitPasswordOnHost bool
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
	topo := c.Config.Topology
	inv := NornirSimpleInventory{
		Kinds:  make(map[string]*NornirSimpleInventoryKindProps),
		Nodes:  make(map[string][]*NornirSimpleInventoryNode),
		Groups: make(map[string][]*NornirSimpleInventoryNode),
	}

	platformNameSchema := os.Getenv(clabconstants.ClabEnvNornirPlatformNameSchema)

	for _, n := range c.Nodes {
		cfg := n.Config()
		credSrc := topo.GetNodeCredentialsTopologySource(cfg.ShortName)
		emitCredsOnHost := credSrc == clabtypes.CredentialTopologyNode ||
			credSrc == clabtypes.CredentialTopologyGroup

		nornirNode := &NornirSimpleInventoryNode{
			NodeConfig:         cfg,
			EmitUsernameOnHost: emitCredsOnHost,
			EmitPasswordOnHost: emitCredsOnHost,
		}

		for key, value := range cfg.Labels {
			if strings.HasPrefix(key, "nornir-group") {
				nornirNode.NornirGroups = append(nornirNode.NornirGroups, value)
			}
		}

		slices.Sort(nornirNode.NornirGroups)

		inv.Nodes[cfg.Kind] = append(inv.Nodes[cfg.Kind], nornirNode)
	}

	for _, nodes := range inv.Nodes {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ShortName < nodes[j].ShortName
		})
	}

	for kind, nodes := range inv.Nodes {
		c.applyNornirHostEmitFlagsForHeterogeneousCredentials(nodes, topo)
		inv.Kinds[kind] = c.buildNornirKindProps(kind, nodes, platformNameSchema, topo)
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

func (c *CLab) applyNornirHostEmitFlagsForHeterogeneousCredentials(
	nodes []*NornirSimpleInventoryNode,
	topo *clabtypes.Topology,
) {
	if len(nodes) < 2 {
		return
	}

	u0 := nodes[0].Credentials.Username
	p0 := nodes[0].Credentials.Password
	uniformUser := true
	uniformPass := true
	for _, n := range nodes[1:] {
		if n.Credentials.Username != u0 {
			uniformUser = false
		}
		if n.Credentials.Password != p0 {
			uniformPass = false
		}
	}

	if !uniformUser {
		for _, n := range nodes {
			n.EmitUsernameOnHost = true
		}
	}
	if !uniformPass {
		for _, n := range nodes {
			n.EmitPasswordOnHost = true
		}
	}

	anyDef := false
	anyNonDef := false
	for _, n := range nodes {
		s := topo.GetNodeCredentialsTopologySource(n.ShortName)
		if s == clabtypes.CredentialTopologyDefaults {
			anyDef = true
		}
		if s != clabtypes.CredentialTopologyUnset &&
			s != clabtypes.CredentialTopologyDefaults {
			anyNonDef = true
		}
	}
	if anyDef && anyNonDef {
		for _, n := range nodes {
			n.EmitUsernameOnHost = true
			n.EmitPasswordOnHost = true
		}
	}
}

func (c *CLab) buildNornirKindProps(
	kind string,
	nodes []*NornirSimpleInventoryNode,
	platformNameSchema string,
	topo *clabtypes.Topology,
) *NornirSimpleInventoryKindProps {
	props := &NornirSimpleInventoryKindProps{Platform: kind}
	if len(nodes) == 0 {
		return props
	}

	nodeRegEntry := c.Reg.Kind(kind)
	if nodeRegEntry != nil && nodeRegEntry.PlatformAttrs() != nil {
		switch platformNameSchema {
		case "napalm":
			props.Platform = nodeRegEntry.PlatformAttrs().NapalmPlatformName
		case "scrapli":
			props.Platform = nodeRegEntry.PlatformAttrs().ScrapliPlatformName
		}
	}

	kindDef := topo.GetKind(strings.ToLower(kind))
	kindTopoUser := kindDef.Credentials.Username
	kindTopoPass := kindDef.Credentials.Password

	u0 := nodes[0].Credentials.Username
	p0 := nodes[0].Credentials.Password
	uniformUser := true
	uniformPass := true
	for _, n := range nodes[1:] {
		if n.Credentials.Username != u0 {
			uniformUser = false
		}
		if n.Credentials.Password != p0 {
			uniformPass = false
		}
	}

	anyFromDefaults := false
	for _, n := range nodes {
		if topo.GetNodeCredentialsTopologySource(
			n.ShortName,
		) == clabtypes.CredentialTopologyDefaults {
			anyFromDefaults = true
			break
		}
	}

	if kindTopoUser != "" {
		props.Username = kindTopoUser
	} else if uniformUser && u0 != "" && !anyFromDefaults {
		props.Username = u0
	}

	if kindTopoPass != "" {
		props.Password = kindTopoPass
	} else if uniformPass && p0 != "" && !anyFromDefaults {
		props.Password = p0
	}

	return props
}

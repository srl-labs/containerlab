// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"io"
	"os"
	"sort"
	"text/template"

	"github.com/srl-labs/containerlab/types"
)

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
	invT := `all:
  children:
{{- range $kind, $nodes := .Nodes}}
    {{$kind}}:
      hosts:
{{- range $nodes}}
        {{.LongName}}:
		{{- if not (eq (index .Labels "ansible-no-host-var") "true") }}
          ansible_host: {{.MgmtIPv4Address}}
		{{- end -}}
{{- end}}
{{- end}}
{{- range $name, $nodes := .Groups}}
    {{$name}}:
      hosts:
{{- range $nodes}}
        {{.LongName}}:
		{{- if not (eq (index .Labels "ansible-no-host-var") "true") }}
          ansible_host: {{.MgmtIPv4Address}}
	    {{- end -}}
{{- end}}
{{- end}}
`

	type inv struct {
		// clab nodes aggregated by their kind
		Nodes map[string][]*types.NodeConfig
		// clab nodes aggregated by user-defined groups
		Groups map[string][]*types.NodeConfig
	}

	i := inv{
		Nodes:  make(map[string][]*types.NodeConfig),
		Groups: make(map[string][]*types.NodeConfig),
	}

	for _, n := range c.Nodes {
		i.Nodes[n.Config().Kind] = append(i.Nodes[n.Config().Kind], n.Config())
		if n.Config().Labels["ansible-group"] != "" {
			i.Groups[n.Config().Labels["ansible-group"]] =
				append(i.Groups[n.Config().Labels["ansible-group"]], n.Config())
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

	t, err := template.New("ansible").Parse(invT)
	if err != nil {
		return err
	}
	err = t.Execute(w, i)
	if err != nil {
		return err
	}
	return err
}

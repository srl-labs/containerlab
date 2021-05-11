package clab

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/srl-labs/containerlab/types"
)

// GenerateInventories generate various inventory files and writes it to a lab location
func (c *CLab) GenerateInventories() error {
	ansibleInvFPath := filepath.Join(c.Dir.Lab, "ansible-inventory.yml")
	f, err := os.Create(ansibleInvFPath)
	if err != nil {
		return err
	}
	if err := c.generateAnsibleInventory(f); err != nil {
		return err
	}
	return nil
}

// generateAnsibleInventory generates and writes ansible inventory file to w
func (c *CLab) generateAnsibleInventory(w io.Writer) error {

	invT :=
		`all:
  children:
{{- range $kind, $nodes := .Nodes}}
    {{$kind}}:
      hosts:
{{- range $nodes}}
        {{.LongName}}:
          ansible_host: {{.MgmtIPv4Address}}
{{- end}}
{{- end}}
`

	type inv struct {
		// clab nodes aggregated by their kind
		Nodes map[string][]*types.Node
		Meta  map[string]string
	}

	i := inv{
		Nodes: make(map[string][]*types.Node),
	}

	for _, n := range c.Nodes {
		i.Nodes[n.Kind] = append(i.Nodes[n.Kind], n)
	}

	// sort nodes by name as they are not sorted originally
	for _, nodes := range i.Nodes {
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

package clab

import (
	"os"
	"path/filepath"
	"text/template"
)

func (c *CLab) GenerateInventories() error {
	if err := c.GenerateAnsibleInventory(); err != nil {
		return err
	}
	return nil
}

func (c *CLab) GenerateAnsibleInventory() error {

	invFPath := filepath.Join(c.Dir.Lab, "ansible-inventory.yml")
	f, err := os.Create(invFPath)
	if err != nil {
		return err
	}

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
		Nodes map[string][]*Node
		Meta  map[string]string
	}

	i := inv{
		Nodes: make(map[string][]*Node),
	}

	for _, n := range c.Nodes {
		i.Nodes[n.Kind] = append(i.Nodes[n.Kind], n)
	}

	t, err := template.New("ansible").Parse(invT)
	if err != nil {
		return err
	}
	err = t.Execute(f, i)
	if err != nil {
		return err
	}
	return err

}

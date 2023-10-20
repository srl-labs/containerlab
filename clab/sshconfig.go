package clab

import (
	"fmt"
	"os"
	"text/template"
)

// SSHConfigTmpl is the top-level data structure for the
// sshconfig template.
type SSHConfigTmpl struct {
	Nodes        []SSHConfigNodeTmpl
	TopologyName string
}

// SSHConfigNodeTmpl represents values for a single node
// in the sshconfig template.
type SSHConfigNodeTmpl struct {
	Name     string
	Username string
}

// tmplSshConfig is the SSH config template.
const tmplSshConfig = `# Containerlab SSH Config for the {{ .TopologyName }} lab

{{- range  .Nodes }}
Host {{ .Name }}
	{{-  if ne .Username ""}}
	User {{ .Username }}
	{{- end }}
	StrictHostKeyChecking=no 
	UserKnownHostsFile=/dev/null
{{ end }}`

// sshConfigFileTmpl is the template for the ssh config file.
const sshConfigFileTmpl = "/etc/ssh/ssh_config.d/clab-%s.conf"

// RemoveSSHConfig removes the lab specific ssh config file
func (c *CLab) RemoveSSHConfig() error {
	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)
	err := os.Remove(filename)
	// if there is an error, thats not "Not Exists", then return it
	if err != nil && err != os.ErrNotExist {
		return err
	}
	return nil
}

// AddSSHConfig adds the lab specific ssh config file.
func (c *CLab) AddSSHConfig() error {
	tmpl := &SSHConfigTmpl{
		TopologyName: c.Config.Name,
		Nodes:        make([]SSHConfigNodeTmpl, 0, len(c.Nodes)),
	}

	// add the data for all nodes to the template input
	for _, n := range c.Nodes {
		// get the Kind from the KindRegistry and and extract
		// the kind registered Username
		NodeRegistryEntry := c.Reg.Kind(n.Config().Kind)
		nodeData := SSHConfigNodeTmpl{
			Name:     n.Config().LongName,
			Username: NodeRegistryEntry.Credentials().GetUsername(),
		}
		tmpl.Nodes = append(tmpl.Nodes, nodeData)
	}

	t, err := template.New("sshconfig").Parse(tmplSshConfig)
	if err != nil {
		return err
	}

	// resolve the output filename with the topology name
	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	err = t.Execute(f, tmpl)
	if err != nil {
		return err
	}

	return nil
}

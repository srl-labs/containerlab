package clab

import (
	"fmt"
	"os"
	"text/template"
)

// tmplSshConfigData is the top-level data structure for the
// sshconfig template
type tmplSshConfigData struct {
	Nodes        []tmplSshConfigDataNode
	TopologyName string
}

// tmplSshConfigDataNode is the per node structure
// thats handed to the template engine
type tmplSshConfigDataNode struct {
	Name     string
	Hostname string
	Username string
}

// tmplSshConfig is the SSH config template itself
const tmplSshConfig = `# Clab SSH Config for topology {{ .TopologyName }}

{{- range  .Nodes }}
Host {{ .Name }}
	Hostname {{ .HostName }}
	User {{ .Username }}
	StrictHostKeyChecking=no 
	UserKnownHostsFile=/dev/null
{{ end }}`

// sshConfigFileTmpl is the sprintf based string, representing the
// sshconfig filename. It just requres the labname as the fmt.Sprintf argument
const sshConfigFileTmpl = "/etc/ssh/ssh_config.d/clab-%s.conf"

// RemoveSSHConfig removes the lab specific ssh config file
func (c *CLab) RemoveSSHConfig() error {
	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)
	return os.Remove(filename)
}

// DeploySSHConfig deploys the lab specific ssh config file
func (c *CLab) DeploySSHConfig() error {
	// create the struct that holds the template input data
	tmplData := &tmplSshConfigData{
		TopologyName: c.Config.Name,
		Nodes:        make([]tmplSshConfigDataNode, 0, len(c.Nodes)),
	}

	// add the data for all nodes to the template input
	for _, n := range c.Nodes {
		// get the Kind from the KindRegistry and and extract
		// the kind registered Username from there
		NodeRegistryEntry := c.Reg.Kind(n.Config().Kind)
		nodeData := tmplSshConfigDataNode{
			Name:     n.Config().LongName,
			Hostname: n.Config().LongName,
			Username: NodeRegistryEntry.Credentials().GetUsername(),
		}
		tmplData.Nodes = append(tmplData.Nodes, nodeData)
	}

	// parse the template
	t, err := template.New("sshconfig").Parse(tmplSshConfig)
	if err != nil {
		return err
	}

	// completly resolve the output filename with the topology name
	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)

	// open the output file, creating if it does not exist, truncate it otherwise
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	// execute the template and write the result
	// to the open file
	err = t.Execute(f, tmplData)
	if err != nil {
		return err
	}
	return nil
}

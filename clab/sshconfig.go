package clab

import (
	"fmt"
	"os"
	"text/template"
)

type tmplSshConfigDataNode struct {
	Name     string
	HostName string
	Username string
}

type tmplSshConfigData struct {
	Nodes        []tmplSshConfigDataNode
	TopologyName string
}

const tmplSshConfig = `# Clab SSH Config for topology {{ .TopologyName }}

{{- range  .Nodes }}
Host {{ .Name }}
	Hostname {{ .HostName }}
	User {{ .Username }}
	StrictHostKeyChecking=no 
	UserKnownHostsFile=/dev/null
{{ end }}`

const sshConfigFileTmpl = "/etc/ssh/ssh_config.d/clab-%s.conf"

func (c *CLab) DestroySSHConfig() error {
	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)
	return os.Remove(filename)
}

func (c *CLab) DeploySSHConfig() error {
	tmplData := &tmplSshConfigData{
		TopologyName: c.Config.Name,
		Nodes:        make([]tmplSshConfigDataNode, 0, len(c.Nodes)),
	}

	for _, n := range c.Nodes {
		// get the Kind from the KindRegistry and and extract
		// the kind registered Username from there
		NodeRegistryEntry := c.Reg.Kind(n.Config().Kind)
		nodeData := tmplSshConfigDataNode{
			Name:     n.Config().LongName,
			HostName: n.Config().LongName,
			Username: NodeRegistryEntry.Credentials().GetUsername(),
		}
		tmplData.Nodes = append(tmplData.Nodes, nodeData)
	}

	t, err := template.New("sshconfig").Parse(tmplSshConfig)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf(sshConfigFileTmpl, c.Config.Name)

	f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	err = t.Execute(f, tmplData)
	if err != nil {
		return err
	}
	return nil
}

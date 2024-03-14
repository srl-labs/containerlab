package clab

import (
	_ "embed"
	"os"
	"path"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/kinds/kind_registry"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
	"golang.org/x/mod/semver"
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
	Name      string
	Username  string
	SSHConfig *types.SSHConfig
}

// sshConfigTemplate is the SSH config template.
//
//go:embed ssh_config.go.tpl
var sshConfigTemplate string

// removeSSHConfig removes the lab specific ssh config file.
func (c *CLab) removeSSHConfig(topoPaths *types.TopoPaths) error {
	err := os.Remove(topoPaths.SSHConfigPath())
	// if there is an error, thats not "Not Exists", then return it
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// addSSHConfig adds the lab specific ssh config file.
func (c *CLab) addSSHConfig() error {
	sshConfigDir := path.Dir(c.TopoPaths.SSHConfigPath())
	if !utils.FileOrDirExists(sshConfigDir) {
		log.Debugf("ssh config directory %s does not exist, skipping ssh config generation", sshConfigDir)
		return nil
	}

	tmpl := &SSHConfigTmpl{
		TopologyName: c.Config.Name,
		Nodes:        make([]SSHConfigNodeTmpl, 0, len(c.Nodes)),
	}

	// get the ssh client version to determine if are allowed
	// to use the PubkeyAuthentication=unbound
	// which is only available in OpenSSH 8.9+
	// if we fail to parse the version the return value is going to be empty
	sshVersion := utils.GetSSHVersion()

	// add the data for all nodes to the template input
	for _, n := range c.Nodes {
		// get the Kind from the KindRegistry and and extract
		// the kind registered Username
		NodeRegistryEntry := kind_registry.KindRegistryInstance.Kind(n.Config().Kind)
		nodeData := SSHConfigNodeTmpl{
			Name:      n.Config().LongName,
			Username:  NodeRegistryEntry.Credentials().GetUsername(),
			SSHConfig: n.GetSSHConfig(),
		}

		// if we couldn't parse the ssh version we assume we can't use unbound option
		// or if the version is lower than 8.9
		// and the node has the PubkeyAuthentication set to unbound
		// we set it to empty string since it is not supported by the SSH client
		if (sshVersion == "" || semver.Compare("v"+sshVersion, "v8.9") < 0) &&
			nodeData.SSHConfig.PubkeyAuthentication == types.PubkeyAuthValueUnbound {
			nodeData.SSHConfig.PubkeyAuthentication = ""
		}

		tmpl.Nodes = append(tmpl.Nodes, nodeData)
	}

	t, err := template.New("sshconfig").Parse(sshConfigTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(c.TopoPaths.SSHConfigPath())
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

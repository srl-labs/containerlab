package core

import (
	_ "embed"
	"os"
	"path"
	"text/template"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
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
	SSHConfig *clabtypes.SSHConfig
}

// sshConfigTemplate is the SSH config template.
//
//go:embed assets/ssh_config.go.tpl
var sshConfigTemplate string

// RemoveSSHConfig removes the lab specific ssh config file.
func (c *CLab) RemoveSSHConfig(topoPaths *clabtypes.TopoPaths) error {
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
	if !clabutils.FileOrDirExists(sshConfigDir) {
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
	sshVersion := clabutils.GetSSHVersion()

	// add the data for all nodes to the template input
	for _, n := range c.Nodes {
		// get the Kind from the KindRegistry and extract
		// the kind registered Username
		NodeRegistryEntry := c.Reg.Kind(n.Config().Kind)
		nodeData := SSHConfigNodeTmpl{
			Name:      n.Config().LongName,
			Username:  NodeRegistryEntry.GetCredentials().GetUsername(),
			SSHConfig: n.GetSSHConfig(),
		}

		// if we couldn't parse the ssh version we assume we can't use unbound option
		// or if the version is lower than 8.9
		// and the node has the PubkeyAuthentication set to unbound
		// we set it to empty string since it is not supported by the SSH client
		if (sshVersion == "" || semver.Compare("v"+sshVersion, "v8.9") < 0) &&
			nodeData.SSHConfig.PubkeyAuthentication ==
				clabtypes.PubkeyAuthValueUnbound {
			nodeData.SSHConfig.PubkeyAuthentication = ""
		}

		tmpl.Nodes = append(tmpl.Nodes, nodeData)
	}

	t, err := template.New("sshconfig").Parse(sshConfigTemplate)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(
		c.TopoPaths.SSHConfigPath(),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		clabconstants.PermissionsFileDefault,
	)
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

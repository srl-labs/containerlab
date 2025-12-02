package core

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	"gopkg.in/yaml.v2"
)

func (c *CLab) Save(
	ctx context.Context,
) error {
	err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return err
	}

	// Load credentials from ansible-inventory.yml for Nokia SROS/SRSIM nodes only
	credsMap := make(map[string]*NodeCredentials)

	// Check if we have any Nokia SROS/SRSIM nodes
	hasTargetNodes := false
	for _, node := range c.Nodes {
		nodeKind := node.Config().Kind
		if nodeKind == "nokia_sros" || nodeKind == "nokia_srsim" {
			hasTargetNodes = true
			break
		}
	}

	// Only read ansible-inventory if we have target nodes
	if hasTargetNodes {
		inventoryPath := c.TopoPaths.AnsibleInventoryFileAbsPath()
		if data, err := os.ReadFile(inventoryPath); err == nil {
			var inventoryYAML AnsibleInventoryYAML
			if err := yaml.Unmarshal(data, &inventoryYAML); err == nil {
				// Process both nokia_sros and nokia_srsim
				for _, kind := range []string{"nokia_sros", "nokia_srsim"} {
					if group, ok := inventoryYAML.All.Children[kind]; ok {
						c.loadGroupCredentials(kind, group, credsMap)
					}
				}
			}
		}
	}

	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))

	for _, node := range c.Nodes {
		go func(node clabnodes.Node) {
			defer wg.Done()

			nodeKind := node.Config().Kind
			nodeName := node.Config().ShortName

			// For Nokia SROS/SRSIM nodes, use NETCONF with credentials from ansible-inventory.yml
			if nodeKind == "nokia_sros" || nodeKind == "nokia_srsim" {
				if creds, ok := credsMap[nodeName]; ok {
					err := c.saveNetconfConfig(
						ctx,
						node,
						creds.Username,
						creds.Password,
						"nokia_sros",
					)
					if err != nil {
						log.Errorf("Failed to save config for %s: %v", nodeName, err)
					}
					return
				}
			}

			// For all other nodes, use default SaveConfig behavior
			err := node.SaveConfig(ctx)
			if err != nil {
				log.Errorf("Failed to save config for %s: %v", nodeName, err)
			}
		}(node)
	}

	wg.Wait()

	return nil
}

// loadGroupCredentials loads credentials for a specific node kind from ansible inventory.
func (c *CLab) loadGroupCredentials(
	kind string,
	group AnsibleInventoryGroup,
	credsMap map[string]*NodeCredentials,
) {
	// Group-level credentials (default for all hosts)
	groupUser := group.Vars.AnsibleUser
	groupPass := group.Vars.AnsiblePassword

	if groupUser != "" && groupPass != "" {
		log.Infof("Using credentials from ansible-inventory.yml for %s nodes (user: %s)", kind, groupUser)
	}

	for nodeName, hostVars := range group.Hosts {
		// Host-level credentials override group-level
		username := groupUser
		password := groupPass

		if hostVars.AnsibleUser != "" {
			username = hostVars.AnsibleUser
		}
		if hostVars.AnsiblePassword != "" {
			password = hostVars.AnsiblePassword
		}

		if username != "" && password != "" {
			credsMap[nodeName] = &NodeCredentials{
				Username: username,
				Password: password,
			}
			if hostVars.AnsibleUser != "" || hostVars.AnsiblePassword != "" {
				log.Debugf("Loaded host-specific credentials for %s node %s (user: %s)", kind, nodeName, username)
			} else {
				log.Debugf("Loaded group credentials for %s node %s", kind, nodeName)
			}
		}
	}
}

// NodeCredentials holds username and password for a node.
type NodeCredentials struct {
	Username string
	Password string
}

// AnsibleInventoryYAML represents the structure of the generated ansible-inventory.yml file.
type AnsibleInventoryYAML struct {
	All struct {
		Children map[string]AnsibleInventoryGroup `yaml:"children"`
	} `yaml:"all"`
}

// AnsibleInventoryGroup represents a group in the ansible inventory YAML.
type AnsibleInventoryGroup struct {
	Vars  AnsibleInventoryVars            `yaml:"vars"`
	Hosts map[string]AnsibleInventoryHost `yaml:"hosts"`
}

// AnsibleInventoryVars represents the vars section in ansible inventory YAML.
type AnsibleInventoryVars struct {
	AnsibleUser     string `yaml:"ansible_user"`
	AnsiblePassword string `yaml:"ansible_password"`
}

// AnsibleInventoryHost represents a host entry in ansible inventory YAML.
type AnsibleInventoryHost struct {
	AnsibleHost     string `yaml:"ansible_host"`
	AnsibleUser     string `yaml:"ansible_user"`
	AnsiblePassword string `yaml:"ansible_password"`
}

// saveNetconfConfig saves configuration using NETCONF with custom credentials.
func (c *CLab) saveNetconfConfig(
	ctx context.Context,
	node clabnodes.Node,
	username, password, platform string,
) error {
	cfg := node.Config()

	// Use management IPv4 address for NETCONF connection
	addr := cfg.MgmtIPv4Address
	if addr == "" {
		addr = cfg.MgmtIPv6Address
	}
	if addr == "" {
		addr = cfg.Fqdn
	}
	if addr == "" {
		addr = cfg.LongName
	}

	// Call netconf SaveRunningConfig with custom credentials
	err := clabnetconf.SaveRunningConfig(
		fmt.Sprintf("[%s]", addr),
		username,
		password,
		platform,
	)
	if err != nil {
		return fmt.Errorf("failed to save config via NETCONF: %w", err)
	}

	log.Infof("saved %s running configuration to startup configuration file\n", cfg.ShortName)
	return nil
}

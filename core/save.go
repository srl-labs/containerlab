package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetconf "github.com/srl-labs/containerlab/netconf"
	clabnodes "github.com/srl-labs/containerlab/nodes"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// InventoryCredsKey is the context key for the inventory credentials map.
	InventoryCredsKey contextKey = "inventoryCredentials"
)

func (c *CLab) Save(
	ctx context.Context,
) error {
	err := clablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return err
	}

	// Read ansible inventory to get credentials for all NETCONF-based nodes
	inventoryPath := c.TopoPaths.AnsibleInventoryFileAbsPath()

	// Load credentials into a map for all NETCONF-based node kinds
	// These node kinds use NETCONF for save operations and need credentials
	credsMap := make(map[string]*AnsibleInventoryCredentials)
	netconfKinds := []string{
		"nokia_sros", "vr-sros", "nokia_srsim", // Nokia SROS variants
		"nokia_srlinux", "srl",                  // Nokia SRL (for consistency, though uses local CLI)
		"c8000",                                 // Cisco 8000
		"xrd",                                   // Cisco XRd
		"vr-vmx", "vr-veos", "vr-sros", "vr-xrv", "vr-xrv9k", "vr-vqfx", "vr-csr", "vr-nxos", // vrnetlab variants
		"vr-ros", "vr-openbsd", "vr-freebsd",   // other vrnetlab variants
	}
	
	for _, nodeKind := range netconfKinds {
		// Check if this kind exists in topology
		hasKind := false
		for _, node := range c.Nodes {
			if node.Config().Kind == nodeKind {
				hasKind = true
				break
			}
		}
		
		if !hasKind {
			continue
		}
		
		// Try to read credentials from inventory
		creds, err := ReadAnsibleInventoryCredentials(inventoryPath, nodeKind)
		if err != nil {
			log.Debugf("Could not read credentials for kind %s from inventory, using defaults", nodeKind)
			// Use default credentials from registry
			if regEntry := c.Reg.Kind(nodeKind); regEntry != nil &&
				regEntry.GetCredentials() != nil {
				credsMap[nodeKind] = &AnsibleInventoryCredentials{
					Username: regEntry.GetCredentials().GetUsername(),
					Password: regEntry.GetCredentials().GetPassword(),
				}
			}
		} else {
			log.Infof("Using credentials from ansible-inventory.yml for %s nodes (user: %s)", nodeKind, creds.Username)
			credsMap[nodeKind] = creds
		}
	}

	// Add credentials map to context
	ctx = context.WithValue(ctx, InventoryCredsKey, credsMap)

	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))

	for _, node := range c.Nodes {
		go func(node clabnodes.Node) {
			defer wg.Done()

			nodeKind := node.Config().Kind

			// For NETCONF-based nodes (except SRL which uses local CLI), intercept and save with custom credentials
			// SROS variants
			if nodeKind == "nokia_sros" || nodeKind == "vr-sros" || nodeKind == "nokia_srsim" {
				creds := credsMap[nodeKind]
				if creds != nil {
					err := c.saveNetconfConfig(ctx, node, creds.Username, creds.Password, "nokia_sros")
					if err != nil {
						log.Errorf("Failed to save config for %s: %v", node.Config().ShortName, err)
					}
					return
				}
			}

			// Cisco c8000
			if nodeKind == "c8000" {
				creds := credsMap[nodeKind]
				if creds != nil {
					err := c.saveNetconfConfig(ctx, node, creds.Username, creds.Password, "cisco_iosxe")
					if err != nil {
						log.Errorf("Failed to save config for %s: %v", node.Config().ShortName, err)
					}
					return
				}
			}

			// Cisco XRd
			if nodeKind == "xrd" {
				creds := credsMap[nodeKind]
				if creds != nil {
					err := c.saveNetconfConfig(ctx, node, creds.Username, creds.Password, "cisco_iosxr")
					if err != nil {
						log.Errorf("Failed to save config for %s: %v", node.Config().ShortName, err)
					}
					return
				}
			}

			// vrnetlab-based nodes (generic handling)
			if isVrnetlabKind(nodeKind) {
				creds := credsMap[nodeKind]
				if creds != nil {
					// vrnetlab nodes use their own SaveConfig which calls GetConfig
					// We can't easily override this without modifying VRNode.SaveConfig
					// So for now, let them use their default SaveConfig which reads from registry
					// TODO: Could be enhanced to pass credentials to VRNode.SaveConfig
				}
			}

			// For all other nodes (including SRL which uses local CLI), use default SaveConfig
			err := node.SaveConfig(ctx)
			if err != nil {
				log.Errorf("Failed to save config for %s: %v", node.Config().ShortName, err)
			}
		}(node)
	}

	wg.Wait()

	return nil
}

// isVrnetlabKind checks if a node kind is a vrnetlab variant.
func isVrnetlabKind(kind string) bool {
	vrnetlabKinds := []string{
		"vr-vmx", "vr-veos", "vr-sros", "vr-xrv", "vr-xrv9k",
		"vr-vqfx", "vr-csr", "vr-nxos", "vr-ros", "vr-openbsd", "vr-freebsd",
	}
	for _, vr := range vrnetlabKinds {
		if kind == vr {
			return true
		}
	}
	return false
}

// saveNetconfConfig saves configuration using NETCONF with credentials from inventory.
// This handles the NETCONF-specific save logic with custom credentials.
func (c *CLab) saveNetconfConfig(
	ctx context.Context,
	node clabnodes.Node,
	username, password, platform string,
) error {
	cfg := node.Config()

	// Use management IPv4 address for NETCONF connection
	// IPv6 addresses need to be enclosed in brackets for netconf library
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

	// Log using the same format as the original SaveConfig methods
	log.Info("Saved running configuration", "node", cfg.ShortName, "addr", cfg.ShortName)
	return nil
}

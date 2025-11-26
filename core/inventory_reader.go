// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// AnsibleInventoryCredentials holds the credentials read from ansible inventory.
type AnsibleInventoryCredentials struct {
	Username string
	Password string
}

// AnsibleInventoryData represents the structure of the ansible-inventory.yml file
// for the purpose of reading credentials.
type AnsibleInventoryData struct {
	All struct {
		Children map[string]AnsibleInventoryKind `yaml:"children"`
	} `yaml:"all"`
}

// AnsibleInventoryKind represents a node kind in the inventory.
type AnsibleInventoryKind struct {
	Vars  AnsibleInventoryVars            `yaml:"vars,omitempty"`
	Hosts map[string]AnsibleInventoryHost `yaml:"hosts,omitempty"`
}

// AnsibleInventoryVars represents the vars section for a kind.
type AnsibleInventoryVars struct {
	User     string `yaml:"ansible_user,omitempty"`
	Password string `yaml:"ansible_password,omitempty"`
}

// AnsibleInventoryHost represents a host entry.
type AnsibleInventoryHost struct {
	Host string `yaml:"ansible_host,omitempty"`
}

// ReadAnsibleInventoryCredentials reads the ansible-inventory.yml file and returns
// credentials for the specified node kind.
func ReadAnsibleInventoryCredentials(
	inventoryPath, nodeKind string,
) (*AnsibleInventoryCredentials, error) {
	// Read the inventory file
	data, err := os.ReadFile(inventoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ansible inventory file: %w", err)
	}

	// Parse the YAML
	var inventory AnsibleInventoryData
	err = yaml.Unmarshal(data, &inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ansible inventory: %w", err)
	}

	// Look for the node kind in the inventory
	kind, exists := inventory.All.Children[nodeKind]
	if !exists {
		return nil, fmt.Errorf("node kind %q not found in ansible inventory", nodeKind)
	}

	// Extract credentials
	creds := &AnsibleInventoryCredentials{
		Username: kind.Vars.User,
		Password: kind.Vars.Password,
	}

	// Validate that we have credentials
	if creds.Username == "" && creds.Password == "" {
		return nil, fmt.Errorf("no credentials found for kind %q in ansible inventory", nodeKind)
	}

	return creds, nil
}

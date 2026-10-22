// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
	"golang.org/x/crypto/ssh"
)

// ExportConfigOverSSH runs command in an SSH exec session at addr (host:port) with password
// authentication and returns its output, trimmed. It serves the kinds whose NOS has no
// scrapligo platform for SaveConfig (MikroTik RouterOS: mikrotik_ros, mikrotik_chr).
func ExportConfigOverSSH(addr, username, password, command string) (string, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lab node, host key unknown
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s via SSH: %w", addr, err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session for %s: %w", addr, err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute %q on %s: %w", command, addr, err)
	}

	exported := strings.TrimSpace(string(output))
	if exported == "" {
		return "", fmt.Errorf("received empty configuration from %s", addr)
	}

	return exported, nil
}

// WriteSavedConfig writes a saved configuration to <LabDir>/<dir>/<file> of the node, the
// path its startup config is read from on the next deployment.
func WriteSavedConfig(
	cfg *clabtypes.NodeConfig,
	dir, file string,
	content []byte,
) (*SaveConfigResult, error) {
	configPath := filepath.Join(cfg.LabDir, dir, file)

	err := os.WriteFile(configPath, content, clabconstants.PermissionsOpen) // skipcq: GO-S2306
	if err != nil {
		return nil, fmt.Errorf(
			"failed to write config by %s path from %s container: %w",
			configPath,
			cfg.ShortName,
			err,
		)
	}

	log.Info("Saved configuration to path", "nodeName", cfg.ShortName, "path", configPath)

	return &SaveConfigResult{ConfigPath: configPath}, nil
}

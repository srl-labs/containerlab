// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"strings"
	"testing"
	"text/template"

	clabtypes "github.com/srl-labs/containerlab/types"
)

// renderSSHConfig renders the embedded ssh_config template for the given nodes and
// returns the produced config text. It mirrors the rendering done in addSSHConfig.
func renderSSHConfig(t *testing.T, nodes []SSHConfigNodeTmpl) string {
	t.Helper()

	tmpl, err := template.New("sshconfig").Parse(sshConfigTemplate)
	if err != nil {
		t.Fatalf("failed to parse ssh config template: %v", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, &SSHConfigTmpl{
		TopologyName: "test",
		Nodes:        nodes,
	}); err != nil {
		t.Fatalf("failed to execute ssh config template: %v", err)
	}

	return sb.String()
}

func TestSSHConfigIdentityFileRendering(t *testing.T) {
	tests := []struct {
		name          string
		node          SSHConfigNodeTmpl
		wantContains  []string
		wantNotContns []string
	}{
		{
			name: "identity file rendered when set",
			node: SSHConfigNodeTmpl{
				Names:        []string{"clab-test-node"},
				Username:     "admin",
				IdentityFile: "/home/user/.ssh/id_node",
				SSHConfig:    &clabtypes.SSHConfig{},
			},
			wantContains: []string{
				"User admin",
				`IdentityFile "/home/user/.ssh/id_node"`,
			},
		},
		{
			name: "no identity file line when unset",
			node: SSHConfigNodeTmpl{
				Names:     []string{"clab-test-node"},
				Username:  "admin",
				SSHConfig: &clabtypes.SSHConfig{},
			},
			wantContains: []string{"User admin"},
			wantNotContns: []string{
				"IdentityFile",
			},
		},
		{
			name: "identity file rendered without username",
			node: SSHConfigNodeTmpl{
				Names:        []string{"clab-test-node"},
				IdentityFile: "~/.ssh/id_node",
				SSHConfig:    &clabtypes.SSHConfig{},
			},
			wantContains: []string{`IdentityFile "~/.ssh/id_node"`},
			wantNotContns: []string{
				"User ",
			},
		},
		{
			name: "identity file with spaces is quoted",
			node: SSHConfigNodeTmpl{
				Names:        []string{"clab-test-node"},
				IdentityFile: "/home/user/my keys/id_node",
				SSHConfig:    &clabtypes.SSHConfig{},
			},
			wantContains: []string{`IdentityFile "/home/user/my keys/id_node"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderSSHConfig(t, []SSHConfigNodeTmpl{tt.node})

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("rendered config missing %q\n--- config ---\n%s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContns {
				if strings.Contains(got, notWant) {
					t.Errorf("rendered config unexpectedly contains %q\n--- config ---\n%s", notWant, got)
				}
			}
		})
	}
}

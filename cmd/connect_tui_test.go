// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"testing"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestConnectCmdHasLabelFlag(t *testing.T) {
	o := GetOptions()

	c, err := connectCmd(o)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Flags().Parse([]string{"--label", "clab-node-kind=linux"}); err != nil {
		t.Fatal(err)
	}

	if len(o.Filter.LabelFilter) != 1 || o.Filter.LabelFilter[0] != "clab-node-kind=linux" {
		t.Errorf("unexpected label filter %v", o.Filter.LabelFilter)
	}
}

func TestFormatConnectPorts(t *testing.T) {
	tests := map[string]struct {
		bindings []*clabtypes.GenericPortBinding
		want     string
	}{
		"none": {want: ""},
		"exposed only, sorted numerically": {
			bindings: []*clabtypes.GenericPortBinding{
				{ContainerPort: 8080, Protocol: "tcp"},
				{ContainerPort: 22, Protocol: "tcp"},
				{ContainerPort: 443, Protocol: "tcp"},
			},
			want: "22/tcp, 443/tcp, 8080/tcp",
		},
		"published on IPv4 and IPv6 is shown once": {
			bindings: []*clabtypes.GenericPortBinding{
				{HostIP: "0.0.0.0", HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
				{HostIP: "::", HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
			},
			want: "18080→80/tcp",
		},
		"published and exposed": {
			bindings: []*clabtypes.GenericPortBinding{
				{ContainerPort: 443, Protocol: "tcp"},
				{HostPort: 18080, ContainerPort: 80, Protocol: "tcp"},
			},
			want: "18080→80/tcp, 443/tcp",
		},
		"no protocol": {
			bindings: []*clabtypes.GenericPortBinding{{ContainerPort: 22}},
			want:     "22",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := formatConnectPorts(tc.bindings); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConnectNode(t *testing.T) {
	node := connectNode(&clabruntime.GenericContainer{
		Names: []string{"/clab-lab1-srl1"},
		Image: "ghcr.io/nokia/srlinux:25.10.1@sha256:abc",
		State: "running",
		Labels: map[string]string{
			clabconstants.NodeKind:     "nokia_srlinux",
			clabconstants.Containerlab: "lab1",
		},
		NetworkSettings: clabruntime.GenericMgmtIPs{
			IPv4addr: "172.20.20.2", IPv4pLen: 24,
			IPv6addr: "3fff:172:20:20::2", IPv6pLen: 64,
		},
		Ports: []*clabtypes.GenericPortBinding{{ContainerPort: 22, Protocol: "tcp"}},
	})

	if node.Name != "clab-lab1-srl1" || node.Kind != "nokia_srlinux" || node.Lab != "lab1" {
		t.Errorf("unexpected identity: %+v", node)
	}

	if node.Image != "nokia/srlinux:25.10.1" {
		t.Errorf("the image is not shortened: %q", node.Image)
	}

	if node.IPv4 != "172.20.20.2/24" || node.IPv6 != "3fff:172:20:20::2/64" ||
		node.Ports != "22/tcp" {
		t.Errorf("unexpected addressing: %+v", node)
	}

	// A container without containerlab labels still gets a usable entry.
	bare := connectNode(&clabruntime.GenericContainer{Names: []string{"other"}})
	if bare.Kind != "linux" || bare.Lab != "-" || bare.IPv4 != "" {
		t.Errorf("unexpected defaults: %+v", bare)
	}
}

//go:build linux && podman
// +build linux,podman

package all

import (
	_ "github.com/srl-labs/containerlab/runtime/docker"
	_ "github.com/srl-labs/containerlab/runtime/ignite"
	_ "github.com/srl-labs/containerlab/runtime/podman"
)

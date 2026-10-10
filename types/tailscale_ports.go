package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/go-connections/nat"
)

const tsPortProto = "/ts"

// SplitDockerAndTailscalePorts separates Docker publish specs from Tailscale Serve mappings.
// Entries ending in /ts are not passed to the container runtime.
func SplitDockerAndTailscalePorts(ports []string) (docker []string, ts []TailscalePort, err error) {
	for _, p := range ports {
		spec := strings.TrimSpace(p)
		if !strings.HasSuffix(strings.ToLower(spec), tsPortProto) {
			docker = append(docker, p)
			continue
		}
		mappings, err := parseTailscalePort(spec[:len(spec)-len(tsPortProto)])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid Tailscale port %q: %w", p, err)
		}
		ts = append(ts, mappings...)
	}
	return docker, ts, nil
}

func parseTailscalePort(spec string) ([]TailscalePort, error) {
	mappings, err := nat.ParsePortSpec(spec + "/tcp")
	if err != nil {
		return nil, err
	}
	ts := make([]TailscalePort, 0, len(mappings))
	for _, m := range mappings {
		if m.Binding.HostIP != "" {
			return nil, fmt.Errorf("listen address is not supported")
		}
		listen, err := strconv.ParseUint(m.Binding.HostPort, 10, 16)
		if err != nil || listen == 0 {
			return nil, fmt.Errorf("listen port is required")
		}
		dest := m.Port.Int()
		if dest == 0 {
			return nil, fmt.Errorf("container port must be non-zero")
		}
		ts = append(ts, TailscalePort{Listen: uint16(listen), Dest: uint16(dest)})
	}
	return ts, nil
}

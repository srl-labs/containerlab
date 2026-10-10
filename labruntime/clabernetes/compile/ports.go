// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package compile

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// typedPort holds typed data about a containerlab port entry.
type typedPort struct {
	Protocol string
	// ExposePort is the pod side port carrying the destination port. processPortDefinition
	// never sets it -- c9s allocates it -- so it is only meaningful on values built from a
	// node's status allocations.
	ExposePort      int64
	DestinationPort int64
}

// normalizePortDefinition reduces a docker style port definition -- "21022:22/tcp" or
// "1.2.3.4:8080:80" -- to the destination port (and protocol) c9s accepts, by dropping
// everything left of the last colon. Definitions that are already destination-only are returned
// unchanged. Pasted containerlab topologies routinely carry the two sided form, so the Topology
// compiler normalizes rather than rejects it; the dropped host side is a port c9s allocates
// itself.
func normalizePortDefinition(portDefinition string) string {
	return portDefinition[strings.LastIndex(portDefinition, ":")+1:]
}

// processPortDefinition accepts a c9s node port definition -- a destination port with an
// optional protocol, i.e. "22" or "5201/udp" -- and returns a typed port. The docker style
// "host:container" form is rejected: the pod side port is an allocation c9s owns, so pinning it
// here cannot work.
func processPortDefinition(portDefinition string) (*typedPort, error) {
	portDefinition = strings.TrimSpace(portDefinition)

	protocol := tcpProtocol

	destinationPort, protocolPart, hasProtocol := strings.Cut(portDefinition, "/")

	if hasProtocol {
		switch strings.ToUpper(protocolPart) {
		case tcpProtocol:
			protocol = tcpProtocol
		case udpProtocol:
			protocol = udpProtocol
		default:
			return nil, fmt.Errorf(
				"%w: port definition %q declares unsupported protocol %q, expected tcp or udp",
				errParse,
				portDefinition,
				protocolPart,
			)
		}
	}

	if strings.Contains(destinationPort, ":") {
		return nil, fmt.Errorf(
			"%w: port definition %q looks like a docker style host:container binding -- declare"+
				" only the destination port (the port the node listens on), clabernetes allocates"+
				" the pod side port itself",
			errParse,
			portDefinition,
		)
	}

	destinationPortAsInt, err := strconv.Atoi(destinationPort)
	if err != nil || destinationPortAsInt < 1 || destinationPortAsInt > maxPort {
		return nil, fmt.Errorf(
			"%w: port definition %q is invalid, expected a destination port between 1 and %d with"+
				" an optional protocol, i.e. \"22\" or \"5201/udp\"",
			errParse,
			portDefinition,
			maxPort,
		)
	}

	return &typedPort{
		Protocol:        protocol,
		DestinationPort: int64(destinationPortAsInt),
	}, nil
}

// parseNetworkModeContainer parses a network-mode value and returns the referenced (primary)
// node name if it is a container network-mode (i.e. "container:node-a" returns "node-a"), or an
// empty string otherwise.
func parseNetworkModeContainer(networkMode string) string {
	if !strings.HasPrefix(networkMode, networkModeContainerPrefix) {
		return ""
	}

	return strings.TrimPrefix(networkMode, networkModeContainerPrefix)
}

// ReservedContainerPathReason reports whether an absolute container path is owned by the
// kubelet or the direct runtime, and why. The returned reason is suitable for a diagnostic.
// A user bind or payload landing on a reserved path either renders an invalid Deployment (the
// kubelet already mounts the path in every container) or silently shadows Pod- or
// runtime-managed content, so both are rejected with a diagnostic instead of failing at the
// API server or misbehaving at runtime.
func reservedContainerPathReason(value string) (string, bool) {
	cleaned := path.Clean(value)

	switch cleaned {
	case "/etc/hosts", "/etc/hostname", "/etc/resolv.conf", "/dev/termination-log":
		return "the kubelet manages this file in every container; it cannot be bind-mounted",
			true
	}

	for _, prefix := range []string{
		"/var/lib/clabernetes",
		"/var/run/clabernetes",
		"/var/run/secrets/kubernetes.io/serviceaccount",
	} {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			return "the direct runtime owns this path inside device containers", true
		}
	}

	return "", false
}

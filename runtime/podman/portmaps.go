//go:build linux && podman
// +build linux,podman

package podman

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	netTypes "github.com/containers/common/libnetwork/types"

	"github.com/charmbracelet/log"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// Reusing parts of the code from podman specgenutil/util.go
// https://github.com/containers/podman/blob/54c630aa0a4dbbddb04ac07b223687aeaa6daefd/pkg/specgenutil/util.go
// Licensed under apache 2.0 license https://github.com/containers/podman/blob/54c630aa0a4dbbddb04ac07b223687aeaa6daefd/LICENSE

// convertPortMap takes a nat.PortMap Docker type and produces a podman-compatible PortMapping.
func (*PodmanRuntime) convertPortMap(_ context.Context, portMap nat.PortMap) ([]netTypes.PortMapping, error) {
	log.Debugf("Method convertPortMap was called with inputs %+v", portMap)
	toReturn := make([]netTypes.PortMapping, 0, len(portMap))
	for port, hostpmap := range portMap {
		var (
			ctrPort  string
			proto    *string
			hostIP   *string
			hostPort *string
		)
		splitProto := strings.Split(string(port), "/")
		switch len(splitProto) {
		case 1:
			// No protocol was provided
		case 2:
			proto = &splitProto[1]
		default:
			return nil, fmt.Errorf("invalid port format - protocol can only be specified once")
		}
		ctrPort = splitProto[0]
		for _, v := range hostpmap {
			hostIP = &v.HostIP
			hostPort = &v.HostPort
			podmanPortMapping, err := parseSplitPort(hostIP, hostPort, ctrPort, proto)
			if err != nil {
				return nil, err
			}
			toReturn = append(toReturn, podmanPortMapping)
		}
	}
	return toReturn, nil
}

func (*PodmanRuntime) convertExpose(_ context.Context, exposePorts nat.PortSet) (map[uint16]string, error) {
	log.Debugf("Method convertExpose was called with inputs %+v", exposePorts)
	toReturn := make(map[uint16]string, len(exposePorts))
	for portProto := range exposePorts {
		proto := portProto.Proto()
		port := portProto.Port()
		// Check for a range
		start, length, err := parseAndValidateRange(port)
		if err != nil {
			return nil, err
		}
		var i uint16
		for i = 0; i < length; i++ {
			portNum := start + i
			protocols, ok := toReturn[portNum]
			if !ok {
				toReturn[portNum] = proto
			} else {
				newProto := strings.Join(append(strings.Split(protocols, ","), strings.Split(proto, ",")...), ",")
				toReturn[portNum] = newProto
			}
		}
	}
	return toReturn, nil
}

func parseSplitPort(hostIP, hostPort *string, ctrPort string, protocol *string) (netTypes.PortMapping, error) {
	newPort := netTypes.PortMapping{}
	if ctrPort == "" {
		return newPort, errors.Errorf("must provide a non-empty container port to publish")
	}
	ctrStart, ctrLen, err := parseAndValidateRange(ctrPort)
	if err != nil {
		return newPort, errors.Wrapf(err, "error parsing container port")
	}
	newPort.ContainerPort = ctrStart
	newPort.Range = ctrLen

	if protocol != nil {
		if *protocol == "" {
			return newPort, errors.Errorf("must provide a non-empty protocol to publish")
		}
		newPort.Protocol = *protocol
	}
	if hostIP != nil {
		// This seems to work with the current clab version (Oct 2021)
		// but need to watch out for "" & 0.0.0.0 handling in the future
		if *hostIP != "" && *hostIP != "0.0.0.0" {
			testIP := net.ParseIP(*hostIP)
			if testIP == nil {
				return newPort, errors.Errorf("cannot parse %q as an IP address", *hostIP)
			}
			newPort.HostIP = testIP.String()
		}
	}
	if hostPort != nil {
		if *hostPort == "" {
			// Set 0 as a placeholder. The server side of Specgen
			// will find a random, open, unused port to use.
			newPort.HostPort = 0
		} else {
			hostStart, hostLen, err := parseAndValidateRange(*hostPort)
			if err != nil {
				return newPort, errors.Wrapf(err, "error parsing host port")
			}
			if hostLen != ctrLen {
				return newPort, errors.Errorf("host and container port ranges have different lengths: %d vs %d", hostLen, ctrLen)
			}
			newPort.HostPort = hostStart
		}
	}

	hport := newPort.HostPort
	log.Debugf("Adding port mapping from %d to %d length %d protocol %q", hport,
		newPort.ContainerPort, newPort.Range, newPort.Protocol)

	return newPort, nil
}

// parseAndValidateRange returns start port, length of range (both uint16) and error.
func parseAndValidateRange(portRange string) (uint16, uint16, error) {
	splitRange := strings.Split(portRange, "-")
	if len(splitRange) > 2 {
		return 0, 0, errors.Errorf("invalid port format - port ranges are formatted as startPort-stopPort")
	}

	if splitRange[0] == "" {
		return 0, 0, errors.Errorf("port numbers cannot be negative")
	}

	startPort, err := parseAndValidatePort(splitRange[0])
	if err != nil {
		return 0, 0, err
	}

	var rangeLen uint16 = 1
	if len(splitRange) == 2 {
		if splitRange[1] == "" {
			return 0, 0, errors.Errorf("must provide ending number for port range")
		}
		endPort, err := parseAndValidatePort(splitRange[1])
		if err != nil {
			return 0, 0, err
		}
		if endPort <= startPort {
			return 0, 0, errors.Errorf("the end port of a range must be higher than the start port - %d is not higher than %d", endPort, startPort)
		}
		// Our range is the total number of ports
		// involved, so we need to add 1 (8080:8081 is
		// 2 ports, for example, not 1)
		rangeLen = endPort - startPort + 1
	}

	return startPort, rangeLen, nil
}

// Turn a single string into a valid U16 port.
func parseAndValidatePort(port string) (uint16, error) {
	num, err := strconv.Atoi(port)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid port number")
	}
	if num < 1 || num > 65535 {
		return 0, errors.Errorf("port numbers must be between 1 and 65535 (inclusive), got %d", num)
	}
	return uint16(num), nil
}

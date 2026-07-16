package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"syscall"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v2"
)

var (
	yamlDupKeyRe   = regexp.MustCompile(`^line (\d+): key "(.*)" already set in map$`)
	yamlBadFieldRe = regexp.MustCompile(`^line (\d+): field (\S+) not found in type \S+$`)
)

// descriptiveYamlError rewrites the yaml-speak of strict unmarshal errors
// ("key already set in map", "field not found in type") into topology file
// terms, one error per finding.
func descriptiveYamlError(typeErr *yaml.TypeError) error {
	errs := make([]error, 0, len(typeErr.Errors))

	for _, msg := range typeErr.Errors {
		if m := yamlDupKeyRe.FindStringSubmatch(msg); m != nil {
			errs = append(errs, fmt.Errorf("line %s: %q is defined more than once", m[1], m[2]))

			continue
		}

		if m := yamlBadFieldRe.FindStringSubmatch(msg); m != nil {
			errs = append(errs, fmt.Errorf(
				"line %s: unknown field %q, consult the release notes to see if it was renamed or removed",
				m[1], m[2]))

			continue
		}

		errs = append(errs, errors.New(msg))
	}

	return errors.Join(errs...)
}

// ValidateTopology runs the topology checks shared by the validate, deploy and
// apply commands, collecting all found errors instead of failing on the first one.
func (c *CLab) ValidateTopology(ctx context.Context) error {
	var errs []error

	if err := c.verifyLinks(ctx); err != nil {
		errs = append(errs, err)
	}

	if err := c.verifyRootNetNSLinks(); err != nil {
		errs = append(errs, err)
	}

	for _, nodeName := range sortedNodeNames(c.Nodes) {
		if err := c.Nodes[nodeName].CheckDeploymentConditions(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if err := c.verifyDuplicateAddresses(); err != nil {
		errs = append(errs, err)
	}

	if err := c.checkHostPortsAvailable(ctx); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// checkHostPortsAvailable checks that the host ports requested by the nodes port
// bindings are not already in use on the host and are not requested by more than
// one node in the topology. Ports published by the lab's own running containers
// are exempt from the probe so that a running lab revalidates cleanly.
func (c *CLab) checkHostPortsAvailable(ctx context.Context) error {
	var errs []error

	claimed := map[string]string{}

	// fetched lazily on the first bound port, topologies without port bindings
	// skip the runtime roundtrip entirely
	var ownPorts map[string]bool

	for name, n := range c.Nodes {
		for port, bindings := range n.Config().PortBindings {
			for _, binding := range bindings {
				if binding.HostPort == "" {
					// the runtime will pick a free port
					continue
				}

				if ownPorts == nil {
					ownPorts = c.ownPublishedPorts(ctx)
				}

				addr := net.JoinHostPort(binding.HostIP, binding.HostPort)
				key := addr + "/" + port.Proto()

				if other, exists := claimed[key]; exists {
					errs = append(errs, fmt.Errorf(
						"host port %s/%s is used by both node %q and node %q",
						addr, port.Proto(), other, name))

					continue
				}

				claimed[key] = name

				// ponytail: exemption matches port/proto only, ignoring the host IP
				if ownPorts[binding.HostPort+"/"+port.Proto()] {
					continue
				}

				if err := checkPortAvailable(port.Proto(), addr); err != nil {
					errs = append(errs, fmt.Errorf(
						"node %q: host port %s/%s %w", name, addr, port.Proto(), err))
				}
			}
		}
	}

	return errors.Join(errs...)
}

// ownPublishedPorts returns the "port/proto" set currently published by the
// lab's own running containers.
func (c *CLab) ownPublishedPorts(ctx context.Context) map[string]bool {
	ports := map[string]bool{}

	if c.Config.Name == "" {
		return ports
	}

	nctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	containers, err := c.ListContainers(nctx, WithListLabName(c.Config.Name))
	if err != nil {
		log.Debugf("could not list the lab's own containers for the port check: %v", err)

		return ports
	}

	for i := range containers {
		for _, p := range containers[i].Ports {
			if p.HostPort != 0 {
				ports[fmt.Sprintf("%d/%s", p.HostPort, p.Protocol)] = true
			}
		}
	}

	return ports
}

// checkPortAvailable probes a host port by briefly binding it. Only conclusive
// failures (port taken, host IP not present on this host) are returned
func checkPortAvailable(proto, addr string) error {
	var err error

	switch proto {
	case "tcp":
		var l net.Listener
		if l, err = net.Listen(proto, addr); err == nil {
			l.Close()
		}
	case "udp":
		var pc net.PacketConn
		if pc, err = net.ListenPacket(proto, addr); err == nil {
			pc.Close()
		}
	default:
		return nil
	}

	switch {
	case errors.Is(err, syscall.EADDRINUSE):
		return errors.New("is already in use on the host")
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		return errors.New("refers to an IP address that is not configured on the host")
	case err != nil:
		log.Debugf("could not probe availability of host port %s/%s: %v", addr, proto, err)
	}

	return nil
}

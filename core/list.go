package core

import (
	"context"
	"fmt"
	"sort"

	"github.com/charmbracelet/log"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// ListContainers lists all containers using provided filter.
func (c *CLab) ListContainers(
	ctx context.Context,
	filter []*types.GenericFilter,
) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, r := range c.Runtimes {
		ctrs, err := r.ListContainers(ctx, filter)
		if err != nil {
			return containers, fmt.Errorf("could not list containers: %v", err)
		}
		containers = append(containers, ctrs...)
	}
	return containers, nil
}

// ListNodesContainers lists all containers based on the nodes stored in clab instance.
func (c *CLab) ListNodesContainers(
	ctx context.Context,
) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, n := range c.Nodes {
		cts, err := n.GetContainers(ctx)
		if err != nil {
			return containers, fmt.Errorf(
				"could not get container for node %s: %v",
				n.Config().LongName, err,
			)
		}

		containers = append(containers, cts...)
	}

	return containers, nil
}

// ListNodesContainersIgnoreNotFound lists all containers based on the nodes stored in clab
// instance, ignoring errors for non found containers.
func (c *CLab) ListNodesContainersIgnoreNotFound(
	ctx context.Context,
) ([]runtime.GenericContainer, error) {
	var containers []runtime.GenericContainer

	for _, n := range c.Nodes {
		cts, err := n.GetContainers(ctx)
		if err != nil {
			continue
		}
		containers = append(containers, cts...)
	}

	return containers, nil
}

// ListContainerInterfaces list interfaces of the given container.
func (c *CLab) ListContainerInterfaces(
	ctx context.Context,
	container runtime.GenericContainer,
) (*types.ContainerInterfaces, error) {
	containerInterfaces := types.ContainerInterfaces{}

	if len(container.Names) > 0 {
		containerInterfaces.ContainerName = container.Names[0]
	}

	// Retrieve the path to the container network NS
	nodeNsPath, err := container.Runtime.GetNSPath(ctx, containerInterfaces.ContainerName)
	if err != nil {
		return nil, err
	}

	// Get network NS handle
	var containerNsHandle netns.NsHandle
	if nodeNsPath != "" {
		// Get the handle for the container network NS
		containerNsHandle, err = netns.GetFromPath(nodeNsPath)
		if err != nil {
			return nil, fmt.Errorf("unable to get container network NS handle: %w", err)
		}
	} else if container.Runtime.GetName() == "podman" {
		// Network NS path is empty and the runtime is Podman -> host network mode
		// Manually get the handle for the root network namespace
		containerNsHandle, err = netns.Get()
		if err != nil {
			return nil, fmt.Errorf("unable to get root network NS handle: %w", err)
		}
	} else {
		log.Warnf("Container %v has no namespace set, skipping!", containerInterfaces.ContainerName)
		containerInterfaces.Interfaces = make([]*types.ContainerInterfaceDetails, 0)
		return &containerInterfaces, nil
	}

	// Get Netlink handle in container network NS
	netlinkHandle, err := netlink.NewHandleAt(containerNsHandle)
	if err != nil {
		return nil, fmt.Errorf("unable to enter container network NS: %w", err)
	}

	interfaces, err := netlinkHandle.LinkList()
	if err != nil {
		return nil, fmt.Errorf("unable to list network interfaces: %w", err)
	}

	containerInterfaces.Interfaces = make([]*types.ContainerInterfaceDetails, 0, len(interfaces))

	for _, iface := range interfaces {
		ifaceDetails := types.ContainerInterfaceDetails{}
		ifaceDetails.InterfaceName = iface.Attrs().Name
		ifaceDetails.InterfaceAlias = iface.Attrs().Alias
		ifaceDetails.InterfaceMTU = iface.Attrs().MTU
		ifaceDetails.InterfaceMAC = iface.Attrs().HardwareAddr.String()
		ifaceDetails.InterfaceIndex = iface.Attrs().Index
		ifaceDetails.InterfaceType = iface.Type()
		ifaceDetails.InterfaceState = iface.Attrs().OperState.String()
		log.Debugf("Interface info: %+v", ifaceDetails)

		containerInterfaces.Interfaces = append(containerInterfaces.Interfaces, &ifaceDetails)
	}
	log.Debugf("Fetched %v interfaces for %v", len(interfaces), containerInterfaces.ContainerName)

	return &containerInterfaces, nil
}

// ListContainersInterfaces list interfaces of all given containers.
func (c *CLab) ListContainersInterfaces(
	ctx context.Context,
	containers []runtime.GenericContainer,
) ([]*types.ContainerInterfaces, error) {
	containerInterfaces := make([]*types.ContainerInterfaces, 0, len(containers))

	for _, cont := range containers {
		cIfs, err := c.ListContainerInterfaces(ctx, cont)
		if err != nil {
			return nil, fmt.Errorf(
				"error getting container interfaces for %v: %w",
				cIfs.ContainerName,
				err,
			)
		}

		if len(cIfs.Interfaces) > 0 {
			sort.Slice(cIfs.Interfaces, func(i, j int) bool {
				return cIfs.Interfaces[i].InterfaceName < cIfs.Interfaces[j].InterfaceName
			})
		} else {
			log.Warnf("No interfaces found for container %v", cIfs.ContainerName)
		}
		containerInterfaces = append(containerInterfaces, cIfs)
	}

	if len(containerInterfaces) == len(containers) {
		sort.Slice(containerInterfaces, func(i, j int) bool {
			return containerInterfaces[i].ContainerName < containerInterfaces[j].ContainerName
		})
	} else {
		return nil, fmt.Errorf(
			"could not retrieve retrieve interfaces for all containers, expected %v, got %v",
			len(containers),
			len(containerInterfaces),
		)
	}

	return containerInterfaces, nil
}

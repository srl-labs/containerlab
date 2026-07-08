package core

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// ResolveNetemTarget picks the netns + interface `tools netem` should act on for
// containerName:iface — a link-owned namespace if a link redirects impairment
// (e.g. veth-stitch), else the container's own netns.
func ResolveNetemTarget(
	ctx context.Context,
	runtimeName string,
	timeout time.Duration,
	containerName, iface string,
) (*clablinks.NetemTarget, error) {
	_, rinit, err := RuntimeInitializer(runtimeName)
	if err != nil {
		return nil, err
	}

	rt := rinit()
	if err := rt.Init(
		clabruntime.WithConfig(&clabruntime.RuntimeConfig{Timeout: timeout}),
	); err != nil {
		return nil, err
	}

	if target, err := linkNetemTarget(ctx, rt, runtimeName, timeout, containerName, iface); err != nil {
		return nil, err
	} else if target != nil {
		return target, nil
	}

	nsPath, err := rt.GetNSPath(ctx, containerName)
	if err != nil {
		return nil, err
	}

	return &clablinks.NetemTarget{
		NSPath:      nsPath,
		Iface:       iface,
		DisplayName: containerName,
	}, nil
}

// linkNetemTarget asks each link in the container's topology whether it redirects
// netem for containerName:iface, returning the first override or nil to fall back
// to the container's own netns. The topology is discovered from the container's
// labels (topology file + node name).
func linkNetemTarget(
	ctx context.Context,
	rt clabruntime.ContainerRuntime,
	runtimeName string,
	timeout time.Duration,
	containerName, iface string,
) (*clablinks.NetemTarget, error) {
	cnts, err := rt.ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: containerName},
	})
	if err != nil || len(cnts) != 1 {
		// not a uniquely resolvable container; let the node path surface the error
		return nil, nil //nolint:nilerr
	}

	topoFile := cnts[0].Labels[clabconstants.TopoFile]
	nodeName := cnts[0].Labels[clabconstants.NodeName]
	// multi-component nodes (e.g. multi-slot SR-SIM) reference the root name.
	rootNodeName := cnts[0].Labels[clabconstants.RootNodeName]
	if topoFile == "" || nodeName == "" {
		return nil, nil
	}

	tc, err := NewContainerLab(
		WithTimeout(timeout),
		WithRuntime(runtimeName, &clabruntime.RuntimeConfig{Timeout: timeout}),
		WithTopoPath(topoFile, nil),
	)
	if err != nil {
		// topology no longer loadable; fall back to the node namespace
		log.Debugf("could not load topology %q to check for netem redirects: %v", topoFile, err)
		return nil, nil //nolint:nilerr
	}

	for _, ld := range tc.Config.Topology.Links {
		target, err := ld.Link.ResolveNetemTarget(
			tc.Config.Name, nodeName, rootNodeName, iface)
		if err != nil {
			return nil, err
		}

		if target != nil {
			return target, nil
		}
	}

	return nil, nil
}

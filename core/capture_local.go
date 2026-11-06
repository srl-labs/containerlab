package core

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	clabcapture "github.com/srl-labs/containerlab/capture"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// localCaptureInterface is a lab link interface of a node on this machine.
type localCaptureInterface struct {
	node clabnodes.Node
	info clablabruntime.CaptureInterface
}

// localCaptureInterfaces lists the link interfaces of the lab's nodes the topology defines, with
// the far end of each link, per node name. Host and management network endpoints are the far
// ends of links, not nodes to capture on.
func (c *CLab) localCaptureInterfaces() (map[string][]localCaptureInterface, error) {
	if len(c.Nodes) == 0 {
		return nil, fmt.Errorf("the topology of lab %q is not available; capture reads the "+
			"lab's links from it, pass it with --topo", c.Config.Name)
	}
	if len(c.Links) == 0 {
		if err := c.ResolveLinks(); err != nil {
			return nil, fmt.Errorf("failed to resolve the links of lab %q: %w", c.Config.Name, err)
		}
	}

	indexes := make([]int, 0, len(c.Links))
	for idx := range c.Links {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)

	result := map[string][]localCaptureInterface{}
	for _, idx := range indexes {
		endpoints := c.Links[idx].GetEndpoints()
		for epIdx, endpoint := range endpoints {
			if endpoint.IsNodeless() || endpoint.IsRuntimeDiscovered() ||
				endpoint.GetNode() == nil {
				continue
			}
			name := endpoint.GetNode().GetShortName()
			node, ok := c.Nodes[name]
			if !ok {
				continue
			}

			info := clablabruntime.CaptureInterface{
				Node:      name,
				Interface: endpoint.GetIfaceName(),
			}
			if alias := endpoint.GetIfaceAlias(); alias != info.Interface {
				info.Alias = alias
			}
			if len(endpoints) == 2 {
				peer := endpoints[1-epIdx]
				info.PeerNode = capturePeerNode(peer)
				info.PeerInterface = peer.GetIfaceDisplayName()
			}

			result[name] = append(result[name], localCaptureInterface{node: node, info: info})
		}
	}

	return result, nil
}

// capturePeerNode names the node at the far end of a link: "host" for a host endpoint and
// "mgmt-net" for the management network, whose shared link nodes carry other names once a lab
// was deployed.
func capturePeerNode(peer clablinks.Endpoint) string {
	if peer.IsNodeless() {
		if _, ok := peer.(*clablinks.EndpointBridge); ok {
			return string(clablinks.LinkTypeMgmtNet)
		}

		return string(clablinks.LinkTypeHost)
	}
	if peer.GetNode() == nil {
		return ""
	}

	return peer.GetNode().GetShortName()
}

// lookupLocalNode finds a lab node by its topology name or its container name.
func (c *CLab) lookupLocalNode(name string) (clabnodes.Node, error) {
	if node, ok := c.Nodes[name]; ok {
		return node, nil
	}
	for _, node := range c.Nodes {
		if node.Config().LongName == name {
			return node, nil
		}
	}

	names := make([]string, 0, len(c.Nodes))
	for nodeName := range c.Nodes {
		names = append(names, nodeName)
	}
	slices.Sort(names)

	return nil, fmt.Errorf("node %q is not part of lab %q; nodes: %s", name, c.Config.Name,
		strings.Join(names, ", "))
}

// nodeRuns reports whether a node's network namespace can be entered.
func nodeRuns(ctx context.Context, node clabnodes.Node) bool {
	if node.Config().IsRootNamespaceBased {
		return true
	}

	return clabruntime.ContainerHasJoinableNetns(node.GetContainerStatus(ctx))
}

// localCaptureInterfacesOf implements CaptureInterfaces for Docker and Podman: the requested
// nodes, or every running node.
func (c *CLab) localCaptureInterfacesOf(
	ctx context.Context,
	nodeNames []string,
) ([]clablabruntime.CaptureInterface, error) {
	if err := c.requireLabName(); err != nil {
		return nil, err
	}
	byNode, err := c.localCaptureInterfaces()
	if err != nil {
		return nil, err
	}

	var nodes []clabnodes.Node
	if len(nodeNames) == 0 {
		for _, node := range c.Nodes {
			if nodeRuns(ctx, node) {
				nodes = append(nodes, node)
			}
		}
	} else {
		for _, name := range nodeNames {
			node, err := c.lookupLocalNode(name)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
	}

	result := make([]clablabruntime.CaptureInterface, 0)
	seen := map[string]bool{}
	for _, node := range nodes {
		name := node.Config().ShortName
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, entry := range byNode[name] {
			result = append(result, entry.info)
		}
	}
	sortCaptureInterfaces(result)

	return result, nil
}

func sortCaptureInterfaces(interfaces []clablabruntime.CaptureInterface) {
	sort.SliceStable(interfaces, func(i, j int) bool {
		if interfaces[i].Node != interfaces[j].Node {
			return interfaces[i].Node < interfaces[j].Node
		}

		return interfaces[i].Interface < interfaces[j].Interface
	})
}

// resolveLocalCaptureTargets checks every target against the node's link interfaces, by name or
// alias, and returns them in request order.
func (c *CLab) resolveLocalCaptureTargets(
	targets []clablabruntime.CaptureTarget,
) ([]clabcapture.LocalTarget, error) {
	byNode, err := c.localCaptureInterfaces()
	if err != nil {
		return nil, err
	}

	resolved := make([]clabcapture.LocalTarget, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		node, err := c.lookupLocalNode(target.Node)
		if err != nil {
			return nil, err
		}
		name := node.Config().ShortName

		infos := make([]clablabruntime.CaptureInterface, 0, len(byNode[name]))
		for _, entry := range byNode[name] {
			infos = append(infos, entry.info)
		}
		sortCaptureInterfaces(infos)
		info, ok := clabcapture.FindInterface(infos, target.Interface)
		if !ok {
			return nil, fmt.Errorf("interface %q of node %q is not a capturable link "+
				"interface; capturable: %s", target.Interface, target.Node,
				clabcapture.DescribeInterfaces(infos))
		}

		key := name + "\x00" + info.Interface
		if seen[key] {
			return nil, fmt.Errorf("interface %q of node %q is requested more than once",
				info.Interface, target.Node)
		}
		seen[key] = true

		resolved = append(resolved, clabcapture.LocalTarget{
			Node:      name,
			Interface: info.Interface,
			Exec:      node.ExecFunction,
		})
	}

	return resolved, nil
}

// captureLocal implements Capture for Docker and Podman through packet sockets in the nodes'
// network namespaces.
func (c *CLab) captureLocal(
	ctx context.Context,
	req clablabruntime.CaptureRequest,
	w io.Writer,
) error {
	if err := c.requireLabName(); err != nil {
		return err
	}
	if req.Name == "" {
		req.Name = c.Config.Name
	}
	if err := clabcapture.ValidateRequest(&req); err != nil {
		return err
	}

	targets, err := c.resolveLocalCaptureTargets(req.Targets)
	if err != nil {
		return err
	}

	return clabcapture.CaptureLocal(ctx, req, targets, w)
}

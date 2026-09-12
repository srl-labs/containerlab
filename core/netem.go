package core

import (
	"context"
	"fmt"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clablinks "github.com/srl-labs/containerlab/links"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// NetemTarget is the netns and interface `tools netem` applies impairment to.
type NetemTarget struct {
	NSPath      string // netns holding Iface
	Iface       string // interface name or altname resolvable within NSPath
	DisplayName string // label for command output
}

// NetemNode is the container `tools netem` operates on: its own netns plus the
// topology identity (lab and node name) read from its labels.
type NetemNode struct {
	NSPath string // the container's own netns

	name      string
	lab, node string // from container labels; empty when unresolvable
}

// TopoIdentity returns the lab and node name recorded in a container's labels.
// A component of a multi-container node (e.g. a multi-slot SR-SIM slot) yields
// its root node name, the name topology links reference.
func TopoIdentity(labels map[string]string) (lab, node string) {
	node = labels[clabconstants.NodeName]
	if root := labels[clabconstants.RootNodeName]; root != "" {
		node = root
	}

	return labels[clabconstants.Containerlab], node
}

// ResolveNetemNode resolves containerName's netns path and topology identity
// via the named runtime. The topology identity is best-effort: it stays empty
// when containerName does not match exactly one container, which disables
// tools-interface
// lookups but leaves the container's own netns usable.
func ResolveNetemNode(
	ctx context.Context,
	runtimeName string,
	timeout time.Duration,
	containerName string,
) (*NetemNode, error) {
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

	n := &NetemNode{name: containerName}

	n.NSPath, err = rt.GetNSPath(ctx, containerName)
	if err != nil {
		return nil, err
	}

	cnts, err := rt.ListContainers(ctx, []*clabtypes.GenericFilter{
		{FilterType: "name", Match: containerName},
	})
	if err == nil && len(cnts) == 1 {
		n.lab, n.node = TopoIdentity(cnts[0].Labels)
	}

	return n, nil
}

// ToolsIfaceFor returns the name of the host-namespace tools interface a link
// published for node:iface (see utils.StitchAltName), or "" when no link
// published one.
func (n *NetemNode) ToolsIfaceFor(iface string) string {
	if _, ok := clablinks.ToolsInterface(n.lab, n.node, iface); !ok {
		return ""
	}

	return clablinks.StitchAltName(n.lab, n.node, iface)
}

// TargetFor returns the netns and interface where impairment of iface must be
// applied: the host-namespace tools interface when a link published one,
// otherwise iface in the container's own netns.
func (n *NetemNode) TargetFor(iface string) (*NetemTarget, error) {
	if toolsIface := n.ToolsIfaceFor(iface); toolsIface != "" {
		current, err := ns.GetCurrentNS()
		if err != nil {
			return nil, err
		}

		return &NetemTarget{
			NSPath:      current.Path(),
			Iface:       toolsIface,
			DisplayName: fmt.Sprintf("%s:%s (host)", n.node, iface),
		}, nil
	}

	return &NetemTarget{
		NSPath:      n.NSPath,
		Iface:       iface,
		DisplayName: n.name,
	}, nil
}

package links

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// LinkVEthStitchedRaw is the raw representation of a veth-stitch link: its two
// endpoints are wired through a dedicated netns and joined with a tc mirred
// egress-redirect, so traffic can be captured and impaired on plain netdevs even
// when the node itself cannot be (e.g. SR-SIM).
type LinkVEthStitchedRaw struct {
	LinkCommonParams `yaml:",inline"`
	Endpoints        stitchEndpoints `yaml:"endpoints"`
}

// stitchEndpoints accepts endpoints in either the brief "node:interface" string
// form or the extended object form.
type stitchEndpoints []*EndpointRaw

func (se *stitchEndpoints) UnmarshalYAML(unmarshal func(any) error) error {
	var briefs []string
	if err := unmarshal(&briefs); err == nil {
		eps := make([]*EndpointRaw, 0, len(briefs))
		for _, b := range briefs {
			node, iface, ok := strings.Cut(b, ":")
			if !ok {
				return fmt.Errorf("invalid veth-stitch endpoint %q, expected node:interface", b)
			}
			eps = append(eps, &EndpointRaw{Node: node, Iface: iface})
		}
		*se = eps

		return nil
	}

	var objs []*EndpointRaw
	if err := unmarshal(&objs); err != nil {
		return err
	}
	*se = objs

	return nil
}

func NewVEthStitchedRawFromVEth(v *LinkVEthRaw) *LinkVEthStitchedRaw {
	return &LinkVEthStitchedRaw{
		LinkCommonParams: v.LinkCommonParams,
		Endpoints:        stitchEndpoints(v.Endpoints),
	}
}

func (*LinkVEthStitchedRaw) GetType() LinkType {
	return LinkTypeVethStitch
}

func (r *LinkVEthStitchedRaw) Resolve(params *ResolveParams) (Link, error) {
	if !isInFilter(params, r.Endpoints) {
		return nil, nil
	}

	if err := validateVEthStitched(r); err != nil {
		return nil, err
	}

	mtu := r.MTU
	if mtu == 0 {
		mtu = clabconstants.DefaultLinkMTU
	}

	node := newVEthStitchNode(VEthStitchNetnsName(params.LabName, r.Endpoints))

	segA, epA, err := buildVEthStitchSegment(params, node, r.Endpoints[0], mtu)
	if err != nil {
		return nil, err
	}

	segB, epB, err := buildVEthStitchSegment(params, node, r.Endpoints[1], mtu)
	if err != nil {
		return nil, err
	}

	l := &LinkVEthStitched{
		segA: segA,
		segB: segB,
		node: node,
		epA:  epA,
		epB:  epB,
	}
	l.MTU = mtu
	l.Vars = normalizeVars(r.Vars)

	return l, nil
}

// vethStitchNode is a nodeless named netns (not a container) that holds the two
// veth ends of a veth-stitch link.
type vethStitchNode struct {
	GenericLinkNode
	netnsName string
	once      sync.Once
	onceErr   error
}

func newVEthStitchNode(netnsName string) *vethStitchNode {
	return &vethStitchNode{
		GenericLinkNode: GenericLinkNode{
			shortname: netnsName,
			endpoints: []Endpoint{},
			nspath:    filepath.Join("/run/netns", netnsName),
		},
		netnsName: netnsName,
	}
}

func (*vethStitchNode) GetLinkEndpointType() LinkEndpointType {
	return LinkEndpointTypeVeth
}

// ensureNetns creates the backing netns once; the two segments may deploy
// concurrently from different node workers.
func (n *vethStitchNode) ensureNetns() error {
	n.once.Do(func() {
		nsPath, err := clabutils.CreateOrGetNamedNetNS(n.netnsName)
		if err != nil {
			n.onceErr = err
			return
		}
		n.nspath = nsPath
	})

	return n.onceErr
}

func (n *vethStitchNode) AddLinkToContainer(
	ctx context.Context,
	link netlink.Link,
	f func(ns.NetNS) error,
) error {
	if err := n.ensureNetns(); err != nil {
		return err
	}

	return n.GenericLinkNode.AddLinkToContainer(ctx, link, f)
}

func (n *vethStitchNode) ExecFunction(ctx context.Context, f func(ns.NetNS) error) error {
	if err := n.ensureNetns(); err != nil {
		return err
	}

	return n.GenericLinkNode.ExecFunction(ctx, f)
}

// LinkVEthStitched is the resolved veth-stitch link: two veth segments
// (nodeA<->netns, nodeB<->netns) whose in-netns ends are joined by a tc stitch.
type LinkVEthStitched struct {
	LinkCommonParams

	segA *LinkVEth
	segB *LinkVEth
	node *vethStitchNode

	// the two netns-side endpoints that get stitched together
	epA Endpoint
	epB Endpoint
}

func (*LinkVEthStitched) GetType() LinkType {
	return LinkTypeVethStitch
}

func (l *LinkVEthStitched) GetEndpoints() []Endpoint {
	return []Endpoint{l.segA.Endpoints[0], l.segB.Endpoints[0]}
}

// Deploy is idempotent; used by apply/tools flows. The normal deploy path relies
// on the node workers deploying the segments and the post-deploy pass calling Stitch.
func (l *LinkVEthStitched) Deploy(ctx context.Context, _ Endpoint) error {
	if err := l.segA.Deploy(ctx, l.segA.Endpoints[0]); err != nil {
		return err
	}

	if err := l.segB.Deploy(ctx, l.segB.Endpoints[0]); err != nil {
		return err
	}

	return l.Stitch(ctx)
}

// Stitch bidirectionally joins the two in-netns interfaces; run after both
// segments are deployed.
func (l *LinkVEthStitched) Stitch(ctx context.Context) error {
	return l.node.ExecFunction(ctx, func(ns.NetNS) error {
		if err := stitch(l.epA, l.epB); err != nil {
			return err
		}

		return stitch(l.epB, l.epA)
	})
}

func (l *LinkVEthStitched) Remove(ctx context.Context) error {
	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}

	if err := l.segA.Remove(ctx); err != nil {
		log.Debug(err)
	}

	if err := l.segB.Remove(ctx); err != nil {
		log.Debug(err)
	}

	// deleting the netns also tears down any interfaces and tc rules left in it
	if err := netns.DeleteNamed(l.node.netnsName); err != nil {
		log.Debugf("failed to delete veth-stitch netns %q: %v", l.node.netnsName, err)
	}

	l.DeploymentState = LinkDeploymentStateRemoved

	return nil
}

// buildVEthStitchSegment builds one veth segment: the real node endpoint plus a
// far end in the stitch netns, named after the facing node.
func buildVEthStitchSegment(
	params *ResolveParams,
	node *vethStitchNode,
	nodeEpRaw *EndpointRaw,
	mtu int,
) (*LinkVEth, Endpoint, error) {
	seg := NewLinkVEth()
	seg.MTU = mtu

	nodeEp, err := nodeEpRaw.Resolve(params, seg)
	if err != nil {
		return nil, nil, err
	}

	// nodeless (EndpointHost) so the veth deploy pushes it into the netns right
	// after the node end, without waiting for a peer worker.
	farEp := NewEndpointHost(NewEndpointGeneric(node, nodeEpRaw.Node, seg))
	seg.Endpoints = []Endpoint{nodeEp, farEp}

	if err := node.AddEndpoint(farEp); err != nil {
		return nil, nil, err
	}

	return seg, farEp, nil
}

func validateVEthStitched(r *LinkVEthStitchedRaw) error {
	if len(r.Endpoints) != 2 {
		return fmt.Errorf("veth-stitch links require exactly two endpoints, got %d", len(r.Endpoints))
	}

	if r.Endpoints[0].Node == r.Endpoints[1].Node {
		return fmt.Errorf(
			"veth-stitch links cannot start and end on the same node %q",
			r.Endpoints[0].Node,
		)
	}

	if len(r.IPv4) > 0 || len(r.IPv6) > 0 {
		return fmt.Errorf("veth-stitch links cannot carry ipv4/ipv6 (transparent L2 stitch)")
	}

	for _, ep := range r.Endpoints {
		// the node name becomes the in-netns interface name
		if !IsValidInterfaceName(ep.Node) {
			return fmt.Errorf(
				"node name %q cannot be used as a veth-stitch interface name (max 15 chars, no '/' or spaces)",
				ep.Node,
			)
		}

		if ep.MAC != "" || ep.IPv4 != "" || ep.IPv6 != "" {
			return fmt.Errorf(
				"veth-stitch endpoint %s:%s cannot carry mac/ipv4/ipv6 (transparent L2 stitch)",
				ep.Node, ep.Iface,
			)
		}
	}

	return nil
}

// ResolveNetemTarget redirects netem into the stitch netns when node:iface is one
// of this link's endpoints, targeting the interface named after the other node
// (egress-of-iface semantics); nil otherwise.
func (r *LinkVEthStitchedRaw) ResolveNetemTarget(
	labName, node, rootNode, iface string,
) (*NetemTarget, error) {
	if len(r.Endpoints) != 2 {
		return nil, nil
	}

	for i, ep := range r.Endpoints {
		if ep.Iface != iface || (ep.Node != node && ep.Node != rootNode) {
			continue
		}

		other := r.Endpoints[(i+1)%2]
		netnsName := VEthStitchNetnsName(labName, r.Endpoints)

		nsPath, err := clabutils.GetNamedNetNS(netnsName)
		if err != nil {
			return nil, fmt.Errorf(
				"veth-stitch namespace %q for %s:%s not found (is the lab deployed?): %w",
				netnsName, node, iface, err)
		}

		return &NetemTarget{
			NSPath:      nsPath,
			Iface:       other.Node,
			DisplayName: fmt.Sprintf("%s:%s (veth-stitch)", node, iface),
		}, nil
	}

	return nil, nil
}

// VEthStitchNetnsName returns a deterministic, lab-scoped name for a link's stitch
// netns. Exported so tooling (tools netem) can locate it.
func VEthStitchNetnsName(labName string, eps []*EndpointRaw) string {
	seed := fmt.Sprintf("%s\x00%s/%s\x00%s/%s",
		labName, eps[0].Node, eps[0].Iface, eps[1].Node, eps[1].Iface)
	sum := sha1.Sum([]byte(seed))

	return fmt.Sprintf("clab-%s-vstitch-%s",
		SanitizeInterfaceName(labName), hex.EncodeToString(sum[:])[:6])
}

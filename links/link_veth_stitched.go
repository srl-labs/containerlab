package links

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabconstants "github.com/srl-labs/containerlab/constants"
	"github.com/vishvananda/netlink"
)

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

	l := &LinkVEthStitched{}
	l.MTU = mtu
	l.Vars = normalizeVars(r.Vars)

	segA, epA, err := buildVEthStitchSegment(params, r.Endpoints[0], mtu)
	if err != nil {
		return nil, err
	}

	segB, epB, err := buildVEthStitchSegment(params, r.Endpoints[1], mtu)
	if err != nil {
		return nil, err
	}

	l.segA, l.segB = segA, segB
	l.epA, l.epB = epA, epB

	return l, nil
}

// LinkVEthStitched is the resolved veth-stitch link: two veth segments
// (nodeA<->root-ns, nodeB<->root-ns) whose root-ns ends are joined by a tc stitch.
type LinkVEthStitched struct {
	LinkCommonParams

	segA *LinkVEth
	segB *LinkVEth

	// the two root-ns endpoints that get stitched together
	epA Endpoint
	epB Endpoint
}

func (*LinkVEthStitched) GetType() LinkType {
	return LinkTypeVethStitch
}

func (l *LinkVEthStitched) GetEndpoints() []Endpoint {
	return []Endpoint{l.segA.Endpoints[0], l.segB.Endpoints[0]}
}

// GetRuntimeEndpoints returns the two node-side endpoints.
func (l *LinkVEthStitched) GetRuntimeEndpoints() []Endpoint {
	return []Endpoint{l.segA.Endpoints[0], l.segB.Endpoints[0]}
}

// Deploy creates both veth segments.
func (l *LinkVEthStitched) Deploy(ctx context.Context, _ Endpoint) error {
	if err := l.segA.Deploy(ctx, l.segA.Endpoints[0]); err != nil {
		return err
	}

	return l.segB.Deploy(ctx, l.segB.Endpoints[0])
}

// PostDeploy tags the two root-ns far ends and joins them with tc
func (l *LinkVEthStitched) PostDeploy(_ context.Context) error {
	if err := markFarEnd(l.epA, l.segA.Endpoints[0]); err != nil {
		return err
	}

	if err := markFarEnd(l.epB, l.segB.Endpoints[0]); err != nil {
		return err
	}

	if err := stitch(l.epA, l.epB); err != nil {
		return err
	}

	return stitch(l.epB, l.epA)
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

	l.DeploymentState = LinkDeploymentStateRemoved

	return nil
}

// buildVEthStitchSegment builds one veth segment: the real node endpoint plus a
// root-namespace far end named with a hash.
func buildVEthStitchSegment(
	params *ResolveParams,
	nodeEpRaw *EndpointRaw,
	mtu int,
) (*LinkVEth, Endpoint, error) {
	seg := NewLinkVEth()
	seg.MTU = mtu

	nodeEp, err := nodeEpRaw.Resolve(params, seg)
	if err != nil {
		return nil, nil, err
	}

	// the far end sits in the root namespace (host node) with a short hash name
	// that fits the 15 char name limit.
	farName := stitchFarEndName(params.LabName, nodeEpRaw.Node, nodeEpRaw.Iface)
	farEp := NewEndpointHost(NewEndpointGeneric(GetHostLinkNode(), farName, seg))
	seg.Endpoints = []Endpoint{nodeEp, farEp}

	return seg, farEp, nil
}

func stitchFarEndName(labName, node, iface string) string {
	sum := sha1.Sum([]byte(labName + "\x00" + node + "\x00" + iface))
	return "clab-s-" + hex.EncodeToString(sum[:4]) // 7 + 8 = 15 chars
}

func markFarEnd(farEp, nodeEp Endpoint) error {
	l, err := netlink.LinkByName(farEp.GetIfaceName())
	if err != nil {
		return fmt.Errorf("failed to look up veth-stitch far end %q: %w", farEp.GetIfaceName(), err)
	}

	if err := netlink.LinkDelAltName(l, ownershipAltName(farEp)); err != nil {
		log.Debugf("could not drop ownership altname on %q: %v", farEp.GetIfaceName(), err)
	}

	altName := "clab-stitch-" + SanitizeInterfaceName(nodeEp.GetNode().GetShortName()+"-"+nodeEp.GetIfaceName())
	if err := netlink.LinkAddAltName(l, altName); err != nil && !isAltNameNotSupportedErr(err) {
		return fmt.Errorf("failed to add veth-stitch altname %q: %w", altName, err)
	}

	return nil
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
		if ep.MAC != "" || ep.IPv4 != "" || ep.IPv6 != "" {
			return fmt.Errorf(
				"veth-stitch endpoint %s:%s cannot carry mac/ipv4/ipv6 (transparent L2 stitch)",
				ep.Node, ep.Iface,
			)
		}
	}

	return nil
}

// ResolveNetemTarget redirects netem to the stitch side interface
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

		current, err := ns.GetCurrentNS()
		if err != nil {
			return nil, err
		}

		return &NetemTarget{
			NSPath:      current.Path(),
			Iface:       stitchFarEndName(labName, other.Node, other.Iface),
			DisplayName: fmt.Sprintf("%s:%s (veth-stitch)", node, iface),
		}, nil
	}

	return nil, nil
}

package links

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

type LinkVEthStitchedRaw struct {
	LinkCommonParams `               yaml:",inline"`
	Endpoints        []*EndpointRaw `yaml:"endpoints"`
}

func NewVEthStitchedRawFromVEth(v *LinkVEthRaw) *LinkVEthStitchedRaw {
	return &LinkVEthStitchedRaw{
		LinkCommonParams: v.LinkCommonParams,
		Endpoints:        v.Endpoints,
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

	l := &LinkVEthStitched{
		LinkCommonParams: r.LinkCommonParams,
		labName:          params.LabName,
	}
	l.MTU = mtu
	l.Vars = normalizeVars(l.Vars)

	segA, epA, err := buildVEthStitchSegment(params, l, r.Endpoints[0], mtu)
	if err != nil {
		return nil, err
	}

	segB, epB, err := buildVEthStitchSegment(params, l, r.Endpoints[1], mtu)
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

	lifecycleMutex sync.Mutex

	// since the stitch veth lives in the root ns, we need to ensure altnames
	// are per-lab unique.
	labName string

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

// GetRuntimeEndpoints returns every material endpoint managed by the stitched link.
func (l *LinkVEthStitched) GetRuntimeEndpoints() []Endpoint {
	return []Endpoint{l.segA.Endpoints[0], l.segB.Endpoints[0], l.epA, l.epB}
}

// Deploy creates the veth segment belonging to ep.
func (l *LinkVEthStitched) Deploy(ctx context.Context, ep Endpoint) error {
	l.lifecycleMutex.Lock()
	defer l.lifecycleMutex.Unlock()

	var err error
	switch ep {
	case l.segA.Endpoints[0]:
		err = l.segA.Deploy(ctx, ep)
	case l.segB.Endpoints[0]:
		err = l.segB.Deploy(ctx, ep)
	default:
		return fmt.Errorf("endpoint %s does not belong to link [ %s, %s ]",
			ep, l.segA.Endpoints[0], l.segB.Endpoints[0])
	}

	if err == nil {
		l.DeploymentState = LinkDeploymentStateHalfDeployed
	}

	return err
}

// PostDeploy tags the two root-ns far ends and joins them with tc
func (l *LinkVEthStitched) PostDeploy(_ context.Context) error {
	l.lifecycleMutex.Lock()
	defer l.lifecycleMutex.Unlock()

	if l.DeploymentState == LinkDeploymentStateFullDeployed {
		return nil
	}
	if l.segA.DeploymentState != LinkDeploymentStateFullDeployed ||
		l.segB.DeploymentState != LinkDeploymentStateFullDeployed {
		return fmt.Errorf("cannot stitch veth segments before both are fully deployed")
	}

	if err := markFarEnd(l.epB, l.segA.Endpoints[0], l.labName); err != nil {
		return err
	}

	if err := markFarEnd(l.epA, l.segB.Endpoints[0], l.labName); err != nil {
		return err
	}

	if err := stitch(l.epA, l.epB); err != nil {
		return err
	}

	if err := stitch(l.epB, l.epA); err != nil {
		return err
	}

	l.DeploymentState = LinkDeploymentStateFullDeployed

	return nil
}

func (l *LinkVEthStitched) Remove(ctx context.Context) error {
	l.lifecycleMutex.Lock()
	defer l.lifecycleMutex.Unlock()

	if l.DeploymentState == LinkDeploymentStateRemoved {
		return nil
	}

	err := errors.Join(l.segA.Remove(ctx), l.segB.Remove(ctx))

	l.DeploymentState = LinkDeploymentStateRemoved

	return err
}

// buildVEthStitchSegment builds one veth segment: the real node endpoint plus a
// root-namespace far end named with a hash.
func buildVEthStitchSegment(
	params *ResolveParams,
	owner *LinkVEthStitched,
	nodeEpRaw *EndpointRaw,
	mtu int,
) (*LinkVEth, Endpoint, error) {
	seg := NewLinkVEth()
	seg.MTU = mtu

	nodeEp, err := nodeEpRaw.Resolve(params, owner)
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

func markFarEnd(farEp, nodeEp Endpoint, labName string) error {
	l, err := netlink.LinkByName(farEp.GetIfaceName())
	if err != nil {
		return fmt.Errorf("failed to look up veth-stitch far end %q: %w", farEp.GetIfaceName(), err)
	}

	iface := nodeEp.GetIfaceAlias()
	if iface == "" {
		iface = nodeEp.GetIfaceName()
	}

	altName := StitchAltName(labName, nodeEp.GetNode().GetShortName(), iface)
	if err := netlink.LinkAddAltName(l, altName); err != nil && !isAltNameNotSupportedErr(err) {
		return fmt.Errorf("failed to add veth-stitch altname %q: %w", altName, err)
	}

	return nil
}

func validateVEthStitched(r *LinkVEthStitchedRaw) error {
	if len(r.Endpoints) != 2 {
		return fmt.Errorf("veth-stitch links require exactly two endpoints, got %d", len(r.Endpoints))
	}

	return nil
}

func (r *LinkVEthStitchedRaw) cleanupFilteredLink(
	ctx context.Context,
	labName string,
	nodesFilter []string,
) error {
	return cleanupFilteredVethStitch(ctx, labName, nodesFilter, r.Endpoints)
}

func cleanupFilteredVethStitch(
	ctx context.Context,
	labName string,
	nodesFilter []string,
	endpoints []*EndpointRaw,
) error {
	for _, altName := range filteredVethStitchAltNames(labName, nodesFilter, endpoints) {
		if err := RemoveOwnedInterface(ctx, GetHostLinkNode(), altName); err != nil {
			return fmt.Errorf("failed cleaning up veth-stitch interface %q: %w", altName, err)
		}
	}

	return nil
}

func filteredVethStitchAltNames(
	labName string,
	nodesFilter []string,
	endpoints []*EndpointRaw,
) []string {
	names := make(map[string]struct{})
	for _, ep := range endpoints {
		if ep == nil || ep.Node == "" || ep.Iface == "" ||
			!slices.Contains(nodesFilter, ep.Node) {
			continue
		}
		names[StitchAltName(labName, ep.Node, ep.Iface)] = struct{}{}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)

	return result
}

// StitchAltName returns a unique name for veth-stitch links.
func StitchAltName(labName, node, iface string) string {
	return "clab-stitch-" + clabutils.SanitizeInterfaceName(labName+"-"+node+"-"+iface)
}

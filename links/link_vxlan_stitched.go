package links

import (
	"context"

	"github.com/charmbracelet/log"
)

type VxlanStitched struct {
	LinkCommonParams
	vxlanLink *LinkVxlan
	vethLink  Link
	// the veth does not distinguish between endpoints. but we
	// need to know which endpoint is the one used for
	// stitching therefore we get that endpoint separately
	vethStitchEp Endpoint
}

// NewVxlanStitched constructs a new VxlanStitched object.
func NewVxlanStitched(vxlan *LinkVxlan, veth Link, vethStitchEp Endpoint) *VxlanStitched {
	// init the VxlanStitched struct
	vxlanStitched := &VxlanStitched{
		LinkCommonParams: vxlan.LinkCommonParams,
		vxlanLink:        vxlan,
		vethLink:         veth,
		vethStitchEp:     vethStitchEp,
	}

	return vxlanStitched
}

// DeployWithExistingVeth provisions the stitched vxlan link whilst the
// veth interface does already exist, hence it is not created as part of this
// deployment. The VxLAN interface is created and TC stitch rules are applied.
func (l *VxlanStitched) DeployWithExistingVeth(ctx context.Context) error {
	return l.internalDeploy(ctx, nil, true)
}

// Stitch applies only the TC redirect rules to bridge the already-existing
// VxLAN and veth interfaces on the host. Both interfaces must already exist.
// Used during lab deploy where VxLAN is created by host endpoint deploy and
// veth is created by node workers.
func (l *VxlanStitched) Stitch() error {
	err := stitch(l.vxlanLink.localEndpoint, l.vethStitchEp)
	if err != nil {
		return err
	}

	return stitch(l.vethStitchEp, l.vxlanLink.localEndpoint)
}

// Deploy provisions the stitched vxlan link with all its underlying sub-links.
func (l *VxlanStitched) Deploy(ctx context.Context, ep Endpoint) error {
	return l.internalDeploy(ctx, ep, false)
}

func (l *VxlanStitched) PostDeploy(ctx context.Context) error {
	return retryTransientNetlink(ctx, "vxlan-stitch post-deploy", l.Stitch)
}

func (l *VxlanStitched) internalDeploy(
	ctx context.Context,
	ep Endpoint,
	skipVethCreation bool,
) error {
	// deploy the vxlan link
	err := l.vxlanLink.Deploy(ctx, ep)
	if err != nil {
		return err
	}

	// the veth creation might be skipped if it already exists
	if !skipVethCreation {
		err = l.vethLink.Deploy(ctx, ep)
		if err != nil {
			return err
		}
	}

	// unidirectionally stitch the vxlan endpoint to the veth endpoint
	err = stitch(l.vxlanLink.localEndpoint, l.vethStitchEp)
	if err != nil {
		return err
	}

	// unidirectionally stitch the veth endpoint to the vxlan endpoint
	err = stitch(l.vethStitchEp, l.vxlanLink.localEndpoint)
	if err != nil {
		return err
	}

	return nil
}

// Remove deprovisions the stitched vxlan link.
func (l *VxlanStitched) Remove(ctx context.Context) error {
	// remove the veth link piece
	err := l.vethLink.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// remove the vxlan link piece
	err = l.vxlanLink.Remove(ctx)
	if err != nil {
		log.Debug(err)
	}
	// set the links DeploymentState to Removed
	l.DeploymentState = LinkDeploymentStateRemoved
	return nil
}

// GetEndpoints returns the endpoints that are part of the link.
func (l *VxlanStitched) GetEndpoints() []Endpoint {
	return []Endpoint{l.vxlanLink.localEndpoint, l.vxlanLink.remoteEndpoint}
}

func (l *VxlanStitched) GetRuntimeEndpoints() []Endpoint {
	eps := l.vethLink.GetRuntimeEndpoints()
	endpoints := make([]Endpoint, 0, len(eps)+1)
	endpoints = append(endpoints, eps...)
	endpoints = append(endpoints, l.vxlanLink.localEndpoint)
	return endpoints
}

// GetType returns the LinkType enum.
func (*VxlanStitched) GetType() LinkType {
	return LinkTypeVxlanStitch
}

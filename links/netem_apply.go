package links

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clabnetem "github.com/srl-labs/containerlab/netem"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// ApplyLinkImpairments sets the link impairments of the given endpoints on their interfaces.
// Impairments don't survive moving an interface to another namespace, so they have to be set
// again whenever the interfaces of a node are restored.
func ApplyLinkImpairments(ctx context.Context, endpoints []Endpoint) error {
	return applyLinkImpairments(ctx, endpoints, loadNetemModule)
}

func applyLinkImpairments(
	ctx context.Context,
	endpoints []Endpoint,
	loadModule func(context.Context),
) error {
	var errs []error
	moduleLoaded := false

	for _, endpoint := range endpoints {
		netem := endpoint.GetNetem()
		if netem == nil {
			continue
		}

		if !moduleLoaded {
			loadModule(ctx)
			moduleLoaded = true
		}

		if err := applyEndpointImpairment(ctx, endpoint, netem); err != nil {
			errs = append(
				errs,
				fmt.Errorf("failed to set the link impairment of %s: %w", endpoint, err),
			)
		}
	}

	return errors.Join(errs...)
}

func applyEndpointImpairment(
	ctx context.Context,
	endpoint Endpoint,
	netem *Netem,
) error {
	if NetemNeedsVEthStitch(endpoint) {
		log.Warn(
			"Link impairments of this node only take effect on veth-stitch links",
			"node", endpoint.GetNode().GetShortName(),
			"interface", endpoint.GetIfaceName(),
		)
	}

	target := NetemEndpoint(endpoint)

	return target.GetNode().ExecFunction(ctx, func(ns.NetNS) error {
		nodeNS, err := ns.GetCurrentNS()
		if err != nil {
			return err
		}
		defer nodeNS.Close()

		tcnl, err := clabnetem.NewTC(int(nodeNS.Fd()))
		if err != nil {
			return err
		}
		defer func() {
			if err := tcnl.Close(); err != nil {
				log.Debugf("could not close rtnetlink socket: %v", err)
			}
		}()

		// interfaces with names linux doesn't allow are found by their sanitized altname
		netlinkLink, err := netlink.LinkByName(
			clabutils.SanitizeInterfaceName(target.GetIfaceName()))
		if _, notFound := err.(netlink.LinkNotFoundError); notFound {
			// e.g. links to filtered-out nodes or containers stopped outside containerlab
			log.Warn(
				"Skipping the link impairment of a missing interface",
				"node", endpoint.GetNode().GetShortName(),
				"interface", endpoint.GetIfaceName(),
			)

			return nil
		}
		if err != nil {
			return err
		}

		link, err := net.InterfaceByName(netlinkLink.Attrs().Name)
		if err != nil {
			return err
		}

		if err := clabnetem.ReplaceImpairments(
			tcnl,
			link,
			netem.Delay,
			netem.Jitter,
			netem.Loss,
			netem.Rate,
			netem.Corruption,
		); err != nil {
			return err
		}

		log.Info(
			"Set link impairments",
			"node", endpoint.GetNode().GetShortName(),
			"interface", endpoint.GetIfaceName(),
		)

		return nil
	})
}

// loadNetemModule loads the sch_netem kernel module, which some distributions don't load on
// their own.
func loadNetemModule(ctx context.Context) {
	if err := exec.CommandContext(ctx, "modprobe", "sch_netem").Run(); err != nil {
		log.Debug("failed to load the sch_netem kernel module", "err", err)
	}
}

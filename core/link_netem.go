package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnetem "github.com/srl-labs/containerlab/netem"
)

func linksEndpoints(links map[int]clablinks.Link) []clablinks.Endpoint {
	var endpoints []clablinks.Endpoint
	for _, link := range links {
		endpoints = append(endpoints, link.GetEndpoints()...)
	}

	return endpoints
}

// applyLinkImpairments sets the link impairments of the given endpoints on their interfaces.
// Impairments don't survive moving an interface to another namespace, so they have to be set
// again whenever the interfaces of a node are restored.
func applyLinkImpairments(ctx context.Context, endpoints []clablinks.Endpoint) error {
	var errs []error
	moduleLoaded := false

	for _, endpoint := range endpoints {
		netem := endpoint.GetNetem()
		if netem == nil {
			continue
		}

		if !moduleLoaded {
			loadNetemModule(ctx)
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
	endpoint clablinks.Endpoint,
	netem *clablinks.Netem,
) error {
	node := endpoint.GetNode()

	return node.ExecFunction(ctx, func(ns.NetNS) error {
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

		link, err := net.InterfaceByName(endpoint.GetIfaceName())
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
			"node", node.GetShortName(),
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

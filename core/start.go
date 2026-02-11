package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/containernetworking/plugins/pkg/ns"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabutils "github.com/srl-labs/containerlab/utils"
	"github.com/vishvananda/netlink"
)

// StartNodes starts one or more stopped nodes and restores their parked interfaces back into the
// container network namespace.
func (c *CLab) StartNodes(ctx context.Context, nodeNames []string) error {
	nodeNames = resolveLifecycleNodeNames(c.Nodes, nodeNames)
	if len(nodeNames) == 0 {
		return fmt.Errorf("%w: lab has no nodes", claberrors.ErrIncorrectInput)
	}

	return c.withLabLock(func() error {
		if err := c.ResolveLinks(); err != nil {
			return err
		}

		nsProviders := c.namespaceShareProviders()

		for _, nodeName := range nodeNames {
			n, err := c.validateLifecycleNode(ctx, nodeName, nsProviders)
			if err != nil {
				return err
			}

			status := n.GetRuntime().GetContainerStatus(ctx, n.Config().LongName)
			switch status {
			case clabruntime.Running:
				log.Debugf("node %q already running, skipping", nodeName)
				continue
			case clabruntime.Stopped:
				// continue below
			default:
				return fmt.Errorf("node %q container %q not found", nodeName, n.Config().LongName)
			}

			if err := c.startNode(ctx, n); err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *CLab) startNode(ctx context.Context, n clabnodes.Node) error {
	cfg := n.Config()

	// Validate link types up-front to avoid partial restore.
	for _, ep := range n.GetEndpoints() {
		if ep.GetLink().GetType() != clablinks.LinkTypeVEth {
			return fmt.Errorf(
				"node %q lifecycle supports only veth links, got %q for endpoint %s",
				cfg.ShortName,
				ep.GetLink().GetType(),
				ep.String(),
			)
		}
	}

	parkName := parkingNetnsName(cfg.LongName)
	parkPath := filepath.Join("/run/netns", parkName)
	if !clabutils.FileOrDirExists(parkPath) {
		return fmt.Errorf(
			"node %q has no parking netns %q; seamless start requires stopping via containerlab",
			cfg.ShortName,
			parkName,
		)
	}

	parkNS, err := ns.GetNS(parkPath)
	if err != nil {
		return fmt.Errorf("node %q failed opening parking netns: %w", cfg.ShortName, err)
	}
	defer parkNS.Close()

	_, err = n.GetRuntime().StartContainer(ctx, cfg.LongName, n)
	if err != nil {
		return fmt.Errorf("node %q failed starting container: %w", cfg.ShortName, err)
	}

	nodeNSPath, err := n.GetNSPath(ctx)
	if err != nil {
		// Try to keep destroy/inspect operational by repointing back to the parking netns.
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed getting netns path: %w", cfg.ShortName, err)
	}

	nodeNS, err := ns.GetNS(nodeNSPath)
	if err != nil {
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed opening netns: %w", cfg.ShortName, err)
	}
	defer nodeNS.Close()

	// Move interfaces back into the container netns.
	moved, err := moveEndpointsBetweenNetNS(parkNS, nodeNS, n.GetEndpoints(), nil)
	if err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_, _ = moveEndpointsBetweenNetNS(nodeNS, parkNS, moved, func(l netlink.Link, ep clablinks.Endpoint) error {
			_ = netlink.LinkSetDown(l)
			return nil
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed restoring interfaces: %w", cfg.ShortName, err)
	}

	// Bring restored interfaces up.
	if err := setEndpointsUp(nodeNS, moved); err != nil {
		// Attempt rollback to keep the node in a consistent stopped+parked state.
		_, _ = moveEndpointsBetweenNetNS(nodeNS, parkNS, moved, func(l netlink.Link, ep clablinks.Endpoint) error {
			_ = netlink.LinkSetDown(l)
			return nil
		})
		_ = clabutils.LinkContainerNS(parkPath, cfg.LongName)
		_ = n.GetRuntime().StopContainer(ctx, cfg.LongName)
		return fmt.Errorf("node %q failed enabling interfaces: %w", cfg.ShortName, err)
	}

	// Re-run topology exec commands on lifecycle start to restore node-local interface config
	// (for example IP addresses added during deploy exec phase).
	execCollection := clabexec.NewExecCollection()
	if err := n.RunExecFromConfig(ctx, execCollection); err != nil {
		log.Errorf("failed to run exec commands for node %q on lifecycle start: %v", cfg.ShortName, err)
	}
	execCollection.Log()

	return nil
}

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
)

var (
	cleanup     bool
	graceful    bool
	keepMgmtNet bool
)

// destroyCmd represents the destroy command.
var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Short:   "destroy a lab",
	Long:    "destroy a lab based defined by means of the topology definition file\nreference: https://containerlab.dev/cmd/destroy/",
	Aliases: []string{"des"},
	PreRunE: sudoCheck,
	RunE:    destroyFn,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", false, "delete lab directory")
	destroyCmd.Flags().BoolVarP(&graceful, "graceful", "", false,
		"attempt to stop containers before removing")
	destroyCmd.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	destroyCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0,
		"limit the maximum number of workers deleting nodes")
	destroyCmd.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")
	destroyCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
}

func destroyFn(_ *cobra.Command, _ []string) error {
	var err error
	var labs []*clab.CLab
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topos := map[string]struct{}{}

	switch {
	case !all:
		topos[topo] = struct{}{}
	case all:
		// only WithRuntime option is needed to list all containers of a lab
		inspectAllOpts := []clab.ClabOption{
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
			clab.WithTimeout(timeout),
		}

		c, err := clab.NewContainerLab(inspectAllOpts...)
		if err != nil {
			return err
		}
		// list all containerlab containers
		filter := []*types.GenericFilter{{
			FilterType: "label", Match: c.Config.Name,
			Field: labels.Containerlab, Operator: "exists",
		}}
		containers, err := c.ListContainers(ctx, filter)
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			return fmt.Errorf("no containerlab labs were found")
		}
		// get unique topo files from all labs
		for i := range containers {
			topos[containers[i].Labels[labels.TopoFile]] = struct{}{}
		}
	}

	log.Debugf("We got the following topos struct for destroy: %+v", topos)
	for topo := range topos {
		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithTopoPath(topo, varsFile),
			clab.WithNodeFilter(nodeFilter),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
			clab.WithDebug(debug),
		}

		if keepMgmtNet {
			opts = append(opts, clab.WithKeepMgmtNet())
		}

		log.Debugf("going through extracted topos for destroy, got a topo file %v and generated opts list %+v", topo, opts)
		nc, err := clab.NewContainerLab(opts...)
		if err != nil {
			return err
		}

		err = links.SetMgmtNetUnderlayingBridge(nc.Config.Mgmt.Bridge)
		if err != nil {
			return err
		}

		// create management network or use existing one
		// we call this to populate the nc.cfg.mgmt.bridge variable
		// which is needed for the removal of the iptables rules
		if err = nc.CreateNetwork(ctx); err != nil {
			return err
		}

		err = nc.ResolveLinks()
		if err != nil {
			return err
		}

		labs = append(labs, nc)
	}

	var errs []error
	for _, clab := range labs {
		err = destroyLab(ctx, clab)
		if err != nil {
			log.Errorf("Error occurred during the %s lab deletion: %v", clab.Config.Name, err)
			errs = append(errs, err)
		}

		if cleanup {
			err = os.RemoveAll(clab.TopoPaths.TopologyLabDir())
			if err != nil {
				log.Errorf("error deleting lab directory: %v", err)
			}
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("error(s) occurred during the deletion. Check log messages")
	}

	return nil
}

func destroyLab(ctx context.Context, c *clab.CLab) (err error) {
	containers, err := c.ListNodesContainersIgnoreNotFound(ctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return nil
	}

	if maxWorkers == 0 {
		maxWorkers = uint(len(c.Nodes))
	}

	// a set of workers that do not support concurrency
	serialNodes := make(map[string]struct{})
	for _, n := range c.Nodes {
		if n.GetRuntime().GetName() == ignite.RuntimeName {
			serialNodes[n.Config().LongName] = struct{}{}
			// decreasing the num of maxWorkers as they are used for concurrent nodes
			maxWorkers = maxWorkers - 1
		}
	}

	// Serializing ignite workers due to busy device error
	if _, ok := c.Runtimes[ignite.RuntimeName]; ok {
		maxWorkers = 1
	}

	log.Infof("Destroying lab: %s", c.Config.Name)
	c.DeleteNodes(ctx, maxWorkers, serialNodes)

	log.Info("Removing containerlab host entries from /etc/hosts file")
	err = clab.DeleteEntriesFromHostsFile(c.Config.Name)
	if err != nil {
		return fmt.Errorf("error while trying to clean up the hosts file: %w", err)
	}

	// delete lab management network
	if c.Config.Mgmt.Network != "bridge" && !keepMgmtNet {
		log.Debugf("Calling DeleteNet method. *CLab.Config.Mgmt value is: %+v", c.Config.Mgmt)
		if err = c.GlobalRuntime().DeleteNet(ctx); err != nil {
			// do not log error message if deletion error simply says that such network doesn't exist
			if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
				log.Error(err)
			}
		}
	}

	// delete container network namespaces symlinks
	for _, node := range c.Nodes {
		err = node.DeleteNetnsSymlink()
		if err != nil {
			return fmt.Errorf("error while deleting netns symlinks: %w", err)
		}
	}

	// Remove any dangling veths from host netns or bridges
	err = c.VethCleanup(ctx)
	if err != nil {
		return fmt.Errorf("error during veth cleanup procedure, %w", err)
	}
	return err
}

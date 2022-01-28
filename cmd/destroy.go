// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	cleanup     bool
	graceful    bool
	keepMgmtNet bool
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:     "destroy",
	Short:   "destroy a lab",
	Long:    "destroy a lab based defined by means of the topology definition file\nreference: https://containerlab.srlinux.dev/cmd/destroy/",
	Aliases: []string{"des"},
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		var labs []*clab.CLab
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
		}

		if keepMgmtNet {
			opts = append(opts, clab.WithKeepMgmtNet())
		}

		topos := map[string]struct{}{}

		switch {
		case !all:
			topos[topo] = struct{}{}
		case all:
			c, err := clab.NewContainerLab(opts...)
			if err != nil {
				return err
			}
			// list all containerlab containers
			labels := []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "exists"}}
			containers, err := c.ListContainers(ctx, labels)
			if err != nil {
				return err
			}

			if len(containers) == 0 {
				return fmt.Errorf("no containerlab labs were found")
			}
			// get unique topo files from all labs
			for _, cont := range containers {
				topos[cont.Labels["clab-topo-file"]] = struct{}{}
			}
		}

		for topo := range topos {
			opts := append(opts,
				clab.WithTopoFile(topo, varsFile),
			)
			c, err := clab.NewContainerLab(opts...)
			if err != nil {
				return err
			}
			// change to the dir where topo file is located
			// to resolve relative paths of license/configs in ParseTopology
			if err = os.Chdir(filepath.Dir(topo)); err != nil {
				return err
			}

			labs = append(labs, c)
		}

		var errs []error
		for _, clab := range labs {
			err = destroyLab(ctx, clab)
			if err != nil {
				log.Errorf("Error occurred during the %s lab deletion %v", clab.Config.Name, err)
				errs = append(errs, err)
			}
		}
		if len(errs) != 0 {
			return fmt.Errorf("error(s) occurred during the deletion. Check log messages")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().BoolVarP(&cleanup, "cleanup", "", false, "delete lab directory")
	destroyCmd.Flags().BoolVarP(&graceful, "graceful", "", false, "attempt to stop containers before removing")
	destroyCmd.Flags().BoolVarP(&all, "all", "a", false, "destroy all containerlab labs")
	destroyCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0, "limit the maximum number of workers deleting nodes")
	destroyCmd.Flags().BoolVarP(&keepMgmtNet, "keep-mgmt-net", "", false, "do not remove the management network")
}

func destroyLab(ctx context.Context, c *clab.CLab) (err error) {

	labels := []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "="}}
	containers, err := c.ListContainers(ctx, labels)
	if err != nil {
		return err
	}

	var labDir string
	if cleanup {
		labDir = c.Dir.Lab
		err = os.RemoveAll(labDir)
		if err != nil {
			log.Errorf("error deleting lab directory: %v", err)
		}
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
		if n.GetRuntime().GetName() == runtime.IgniteRuntime {
			serialNodes[n.Config().LongName] = struct{}{}
			// decreasing the num of maxWorkers as they are used for concurrent nodes
			maxWorkers = maxWorkers - 1
		}
	}

	// Serializing ignite workers due to busy device error
	if _, ok := c.Runtimes[runtime.IgniteRuntime]; ok {
		maxWorkers = 1
	}

	log.Infof("Destroying lab: %s", c.Config.Name)
	c.DeleteNodes(ctx, maxWorkers, serialNodes)

	log.Info("Removing containerlab host entries from /etc/hosts file")
	err = clab.DeleteEntriesFromHostsFile(c.Config.Name)
	if err != nil {
		return err
	}

	// delete lab management network
	if c.Config.Mgmt.Network != "bridge" && !keepMgmtNet {
		if err = c.GlobalRuntime().DeleteNet(ctx); err != nil {
			// do not log error message if deletion error simply says that such network doesn't exist
			if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
				log.Error(err)
			}
		}
	}
	// delete container network namespaces symlinks
	return c.DeleteNetnsSymlinks()
}

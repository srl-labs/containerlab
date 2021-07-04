// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/runtime/ignite"
	"github.com/srl-labs/containerlab/types"
)

var cleanup bool
var graceful bool

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
			clab.WithDebug(debug),
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt, debug, timeout, graceful),
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
			labels := []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "="}}
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
				clab.WithTopoFile(topo),
				clab.WithGracefulShutdown(graceful),
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
}

func deleteEntriesFromHostsFile(containers []types.GenericContainer, bridgeName string) error {
	if bridgeName == "" {
		return fmt.Errorf("missing bridge name")
	}
	f, err := os.OpenFile("/etc/hosts", os.O_RDWR, 0644) // skipcq: GSC-G302
	if err != nil {
		return err
	}
	defer f.Close()
	data := hostsEntries(containers, bridgeName)
	remainingLines := make([][]byte, 0)
	reader := bufio.NewReader(f)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		found := false
		sLine := strings.Join(strings.Fields(string(line)), " ")
		for _, dl := range strings.Split(string(data), "\n") {
			sdl := strings.Join(strings.Fields(string(dl)), " ")
			if strings.Compare(sLine, sdl) == 0 {
				found = true
				break
			}
		}
		if !found {
			remainingLines = append(remainingLines, line)
		}
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}
	for _, l := range remainingLines {
		_, _ = f.Write(l)
		_, _ = f.Write([]byte("\n"))
	}
	return nil
}

func destroyLab(ctx context.Context, c *clab.CLab) (err error) {

	labels := []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "="}}
	containers, err := c.ListContainers(ctx, labels)
	if err != nil {
		return err
	}

	var labDir string
	for _, ctr := range containers {
		if n, ok := c.Nodes[ctr.ShortID]; ok {
			c.Nodes[ctr.ShortID] = n
			labDir = n.Config().LabDir
		}
	}

	if cleanup {
		labDir = filepath.Dir(labDir)
	}

	if maxWorkers == 0 {
		maxWorkers = uint(len(c.Nodes))
	}

	// Serializing ignite workers due to busy device error
	if rt == ignite.RuntimeName {
		maxWorkers = 1
	}

	log.Infof("Destroying lab: %s", c.Config.Name)
	c.DeleteNodes(ctx, maxWorkers, c.Nodes)

	// remove the lab directories
	if cleanup {
		err = os.RemoveAll(labDir)
		if err != nil {
			log.Errorf("error deleting lab directory: %v", err)
		}
	}

	log.Info("Removing container entries from /etc/hosts file")
	err = deleteEntriesFromHostsFile(containers, c.Config.Mgmt.Network)
	if err != nil {
		return err
	}

	// delete lab management network
	log.Infof("Deleting network '%s'...", c.Config.Mgmt.Network)
	if err = c.GlobalRuntime().DeleteNet(ctx); err != nil {
		// do not log error message if deletion error simply says that such network doesn't exist
		if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
			log.Error(err)
		}

	}
	// delete container network namespaces symlinks
	return c.DeleteNetnsSymlinks()
}

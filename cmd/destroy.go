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
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
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

		switch {
		case !all:
			// stop if not topo file provided and not all labs are requested
			// to be deleted
			if err = topoSet(); err != nil {
				return err
			}
			opts := []clab.ClabOption{
				clab.WithDebug(debug),
				clab.WithTimeout(timeout),
				clab.WithTopoFile(topo),
				clab.WithRuntime(rt, debug, timeout, graceful),
				clab.WithGracefulShutdown(graceful),
			}
			c := clab.NewContainerLab(opts...)

			// Parse topology information
			if err = c.ParseTopology(); err != nil {
				return err
			}
			labs = append(labs, c)
		case all:
			opts := []clab.ClabOption{
				clab.WithDebug(debug),
				clab.WithTimeout(timeout),
				clab.WithRuntime(rt, debug, timeout, graceful),
			}
			c := clab.NewContainerLab(opts...)
			// list all containerlab containers
			containers, err := c.Runtime.ListContainers(ctx, []*types.GenericFilter{{FilterType: "label", Field: "containerlab", Operator: "exists"}})
			if err != nil {
				return fmt.Errorf("could not list containers: %v", err)
			}
			if len(containers) == 0 {
				return fmt.Errorf("no containerlab labs were found")
			}
			// get unique topo files from all labs
			topos := map[string]struct{}{}
			for _, cont := range containers {
				topos[cont.Labels["clab-topo-file"]] = struct{}{}
			}
			for topo := range topos {
				opts := []clab.ClabOption{
					clab.WithDebug(debug),
					clab.WithTimeout(timeout),
					clab.WithTopoFile(topo),
					clab.WithRuntime(rt, debug, timeout, graceful),
					clab.WithGracefulShutdown(graceful),
				}
				c = clab.NewContainerLab(opts...)
				// change to the dir where topo file is located
				// to resolve relative paths of license/configs in ParseTopology
				if err = os.Chdir(filepath.Dir(topo)); err != nil {
					return err
				}

				// Parse topology information
				if err = c.ParseTopology(); err != nil {
					return err
				}
				labs = append(labs, c)
			}
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
	destroyCmd.Flags().UintVarP(&maxWorkers, "max-workers", "", 0, "limit the maximum number of workers deleteing nodes")
}

func deleteEntriesFromHostsFile(containers []types.GenericContainer, bridgeName string) error {
	if bridgeName == "" {
		return fmt.Errorf("missing bridge name")
	}
	f, err := os.OpenFile("/etc/hosts", os.O_RDWR, 0644)
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
	containers, err := c.Runtime.ListContainers(ctx, labels)
	if err != nil {
		return fmt.Errorf("could not list containers: %v", err)
	}
	if len(containers) == 0 {
		return nil
	}

	// get lab directory used by this lab to remove it later if cleanup is used
	var labDir string
	if cleanup {
		labDir = filepath.Dir(containers[0].Labels["clab-node-lab-dir"])
	}

	if maxWorkers == 0 {
		maxWorkers = uint(len(containers))
	}

	log.Infof("Destroying lab: %s", c.Config.Name)
	ctrChan := make(chan *types.GenericContainer)
	wg := new(sync.WaitGroup)
	wg.Add(int(maxWorkers))
	for i := uint(0); i < maxWorkers; i++ {

		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case cont := <-ctrChan:
					if cont == nil {
						log.Debugf("Worker %d terminating...", i)
						return
					}
					//if len(cont.Names) > 0 {
					//	name = strings.TrimLeft(cont.Names[0], "/")
					//}
					err := c.Runtime.DeleteContainer(ctx, cont)
					if err != nil {
						log.Errorf("could not remove container: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
	for _, ctr := range containers {
		ctr := ctr
		ctrChan <- &ctr
	}
	close(ctrChan)

	wg.Wait()

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
	if err = c.Runtime.DeleteNet(ctx); err != nil {
		// do not log error message if deletion error simply says that such network doesn't exist
		if err.Error() != fmt.Sprintf("Error: No such network: %s", c.Config.Mgmt.Network) {
			log.Error(err)
		}

	}
	// delete container network namespaces symlinks
	return c.DeleteNetnsSymlinks()
}

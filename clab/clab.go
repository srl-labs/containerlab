// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/nodes"
	_ "github.com/srl-labs/containerlab/nodes/all"
	"github.com/srl-labs/containerlab/runtime"
	_ "github.com/srl-labs/containerlab/runtime/all"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

type CLab struct {
	Config   *Config
	TopoFile *TopoFile
	m        *sync.RWMutex
	Nodes    map[string]nodes.Node
	Links    map[int]*types.Link
	Runtime  runtime.ContainerRuntime
	Dir      *Directory

	debug            bool
	timeout          time.Duration
	gracefulShutdown bool
}

type Directory struct {
	Lab       string
	LabCA     string
	LabCARoot string
	LabGraph  string
}

type ClabOption func(c *CLab)

func WithDebug(d bool) ClabOption {
	return func(c *CLab) {
		c.debug = d
	}
}

func WithTimeout(dur time.Duration) ClabOption {
	return func(c *CLab) {
		c.timeout = dur
	}
}

func WithRuntime(name string, d bool, dur time.Duration, gracefulShutdown bool) ClabOption {
	return func(c *CLab) {
		// define runtime name.
		// order of preference: cli flag -> env var -> default value of docker
		envN := os.Getenv("CLAB_RUNTIME")
		switch {
		case name != "":
		case envN != "":
			name = envN
		default:
			name = runtime.DockerRuntime
		}

		if rInit, ok := runtime.ContainerRuntimes[name]; ok {
			c.Runtime = rInit()
			err := c.Runtime.Init(
				runtime.WithConfig(&runtime.RuntimeConfig{
					Timeout: dur,
					Debug:   d,
				}),
				runtime.WithMgmtNet(c.Config.Mgmt),
			)
			if err != nil {
				log.Fatalf("failed to init the container runtime: %s", err)
			}
			return
		}
		log.Fatalf("unknown container runtime %q", name)
	}
}

func WithTopoFile(file string) ClabOption {
	return func(c *CLab) {
		if file == "" {
			log.Fatal("provide a path to the clab toplogy file")
		}
		if err := c.GetTopology(file); err != nil {
			log.Fatalf("failed to read topology file: %v", err)
		}

		err := c.initMgmtNetwork()
		if err != nil {
			log.Fatalf("failed to init the management network: %s", err)
		}

	}
}

func WithGracefulShutdown(gracefulShutdown bool) ClabOption {
	return func(c *CLab) {
		c.gracefulShutdown = gracefulShutdown
	}
}

// NewContainerLab function defines a new container lab
func NewContainerLab(opts ...ClabOption) *CLab {
	c := &CLab{
		Config: &Config{
			Mgmt:     new(types.MgmtNet),
			Topology: types.NewTopology(),
		},
		TopoFile: new(TopoFile),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]nodes.Node),
		Links:    make(map[int]*types.Link),
	}

	for _, o := range opts {
		o(c)
	}

	return c
}

// initMgmtNetwork sets management network config
func (c *CLab) initMgmtNetwork() error {
	if c.Config.Mgmt.Network == "" {
		c.Config.Mgmt.Network = dockerNetName
	}
	if c.Config.Mgmt.IPv4Subnet == "" && c.Config.Mgmt.IPv6Subnet == "" {
		c.Config.Mgmt.IPv4Subnet = dockerNetIPv4Addr
		c.Config.Mgmt.IPv6Subnet = dockerNetIPv6Addr
	}

	// init docker network mtu
	if c.Config.Mgmt.MTU == "" {
		m, err := utils.DefaultNetMTU()
		if err != nil {
			log.Warnf("Error occurred during getting the default docker MTU: %v", err)
		}
		c.Config.Mgmt.MTU = m
	}

	return nil
}

func (c *CLab) CreateNodes(ctx context.Context, workers uint) {
	wg := new(sync.WaitGroup)
	wg.Add(int(workers))
	nodesChan := make(chan nodes.Node)
	// start workers
	for i := uint(0); i < workers; i++ {
		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case node, ok := <-nodesChan:
					if node == nil || !ok {
						log.Debugf("Worker %d terminating...", i)
						return
					}
					log.Debugf("Worker %d received node: %+v", i, node.Config())
					// PreDeploy
					err := node.PreDeploy(c.Config.Name, c.Dir.LabCA, c.Dir.LabCARoot)
					if err != nil {
						log.Errorf("failed pre-deploy phase for node %q: %v", node.Config().ShortName, err)
						continue
					}
					// Deploy
					err = node.Deploy(ctx, c.Runtime)
					if err != nil {
						log.Errorf("failed deploy phase for node %q: %v", node.Config().ShortName, err)
						continue
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
	for _, n := range c.Nodes {
		nodesChan <- n
	}
	// close channel to terminate the workers
	close(nodesChan)
	// wait for all workers to finish
	wg.Wait()
}

// CreateLinks creates links using the specified number of workers
// `postdeploy` indicates the stage of links creation.
// `postdeploy=true` means the links routine is called after nodes postdeploy tasks
func (c *CLab) CreateLinks(ctx context.Context, workers uint, postdeploy bool) {
	wg := new(sync.WaitGroup)
	wg.Add(int(workers))
	linksChan := make(chan *types.Link)

	log.Debug("creating links...")
	// wire the links between the nodes based on cabling plan
	for i := uint(0); i < workers; i++ {
		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case link := <-linksChan:
					if link == nil {
						log.Debugf("Link worker %d terminating...", i)
						return
					}
					log.Debugf("Link worker %d received link: %+v", i, link)
					if err := c.CreateVirtualWiring(link); err != nil {
						log.Error(err)
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	for _, link := range c.Links {
		// skip the links of ceos kind
		// ceos containers need to be restarted in the postdeploy stage, thus their data links
		// will get recreated after post-deploy stage
		if !postdeploy {
			if link.A.Node.Kind == "ceos" || link.B.Node.Kind == "ceos" {
				continue
			}
			linksChan <- link
		} else {
			// postdeploy stage
			// create ceos links that were skipped during original links creation
			if link.A.Node.Kind == "ceos" || link.B.Node.Kind == "ceos" {
				linksChan <- link
			}
		}
	}
	// close channel to terminate the workers
	close(linksChan)
	// wait for all workers to finish
	wg.Wait()
}

func (c *CLab) DeleteNodes(ctx context.Context, workers uint, containers []types.GenericContainer) {
	wg := new(sync.WaitGroup)

	ctrChan := make(chan *types.GenericContainer)
	wg.Add(int(workers))
	for i := uint(0); i < workers; i++ {
		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case cont := <-ctrChan:
					if cont == nil {
						log.Debugf("Worker %d terminating...", i)
						return
					}
					err := c.Runtime.DeleteContainer(ctx, cont)
					if err != nil {
						log.Errorf("could not remove container '%s': %v", cont.ID, err)
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

}

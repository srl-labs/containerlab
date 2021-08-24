// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"fmt"
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
	Config        *Config
	TopoFile      *TopoFile
	m             *sync.RWMutex
	Nodes         map[string]nodes.Node
	Links         map[int]*types.Link
	Runtimes      map[string]runtime.ContainerRuntime
	globalRuntime string
	Dir           *Directory

	timeout time.Duration
}

type Directory struct {
	Lab       string
	LabCA     string
	LabCARoot string
	LabGraph  string
}

type ClabOption func(c *CLab)

func WithTimeout(dur time.Duration) ClabOption {
	return func(c *CLab) {
		c.timeout = dur
	}
}

func WithRuntime(name string, rtconfig *runtime.RuntimeConfig) ClabOption {
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
		c.globalRuntime = name

		if rInit, ok := runtime.ContainerRuntimes[name]; ok {
			r := rInit()
			err := r.Init(
				runtime.WithConfig(rtconfig),
				runtime.WithMgmtNet(c.Config.Mgmt),
			)
			if err != nil {
				log.Fatalf("failed to init the container runtime: %s", err)
			}

			c.Runtimes[name] = r

			return
		}
		log.Fatalf("unknown container runtime %q", name)
	}
}

func WithKeepMgmtNet() ClabOption {
	return func(c *CLab) {
		c.GlobalRuntime().WithKeepMgmtNet()
	}
}

func WithTopoFile(file string) ClabOption {
	return func(c *CLab) {
		if file == "" {
			log.Fatal("provide a path to the clab topology file")
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

// NewContainerLab function defines a new container lab
func NewContainerLab(opts ...ClabOption) (*CLab, error) {
	c := &CLab{
		Config: &Config{
			Mgmt:     new(types.MgmtNet),
			Topology: types.NewTopology(),
		},
		TopoFile: new(TopoFile),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]nodes.Node),
		Links:    make(map[int]*types.Link),
		Runtimes: make(map[string]runtime.ContainerRuntime),
	}

	for _, o := range opts {
		o(c)
	}

	var err error
	if c.TopoFile.path != "" {
		err = c.parseTopology()
	}

	return c, err
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

func (c *CLab) GlobalRuntime() runtime.ContainerRuntime {
	return c.Runtimes[c.globalRuntime]
}

func (c *CLab) CreateNodes(ctx context.Context, maxWorkers uint,
	serialNodes map[string]struct{}) (*sync.WaitGroup, *sync.WaitGroup) {
	staticIPNodes := make(map[string]nodes.Node)
	dynIPNodes := make(map[string]nodes.Node)

	for name, n := range c.Nodes {
		if n.Config().MgmtIPv4Address != "" || n.Config().MgmtIPv6Address != "" {
			staticIPNodes[name] = n
			continue
		}
		dynIPNodes[name] = n
	}
	var staticIPWg *sync.WaitGroup
	var dynIPWg *sync.WaitGroup
	if len(staticIPNodes) > 0 {
		log.Debug("scheduling nodes with static IPs...")
		staticIPWg = c.createNodes(ctx, int(maxWorkers), serialNodes, staticIPNodes)
	}
	if len(dynIPNodes) > 0 {
		log.Debug("scheduling nodes with dynamic IPs...")
		dynIPWg = c.createNodes(ctx, int(maxWorkers), serialNodes, dynIPNodes)
	}
	return staticIPWg, dynIPWg
}

func (c *CLab) createNodes(ctx context.Context, maxWorkers int,
	serialNodes map[string]struct{}, scheduledNodes map[string]nodes.Node) *sync.WaitGroup {
	concurrentChan := make(chan nodes.Node)
	serialChan := make(chan nodes.Node)

	workerFunc := func(i int, input chan nodes.Node, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case node, ok := <-input:
				if node == nil || !ok {
					log.Debugf("Worker %d terminating...", i)
					return
				}
				log.Debugf("Worker %d received node: %+v", i, node.Config())

				// Apply any startup delay
				delay := node.Config().StartupDelay
				if delay > 0 {
					log.Infof("node %q is being delayed for %d seconds", node.Config().ShortName, delay)
					time.Sleep(time.Duration(delay) * time.Second)
				}

				// PreDeploy
				err := node.PreDeploy(c.Config.Name, c.Dir.LabCA, c.Dir.LabCARoot)
				if err != nil {
					log.Errorf("failed pre-deploy phase for node %q: %v", node.Config().ShortName, err)
					continue
				}
				// Deploy
				err = node.Deploy(ctx)
				if err != nil {
					log.Errorf("failed deploy phase for node %q: %v", node.Config().ShortName, err)
					continue
				}

				node.Config().DeploymentStatus = "created"
			case <-ctx.Done():
				return
			}
		}
	}

	numScheduledNodes := len(scheduledNodes)
	if numScheduledNodes < maxWorkers {
		maxWorkers = numScheduledNodes
	}
	wg := new(sync.WaitGroup)

	// start concurrent workers
	wg.Add(int(maxWorkers))
	// it's safe to not check if all nodes are serial because in that case
	// maxWorkers will be 0
	for i := 0; i < maxWorkers; i++ {
		go workerFunc(i, concurrentChan, wg)
	}

	// start the serial worker
	if len(serialNodes) > 0 {
		wg.Add(1)
		go workerFunc(maxWorkers, serialChan, wg)
	}

	// send nodes to workers
	for _, n := range scheduledNodes {
		if _, ok := serialNodes[n.Config().LongName]; ok {
			// delete the entry to avoid starting a serial worker in the
			// case of dynamic IP nodes scheduling
			delete(serialNodes, n.Config().LongName)
			serialChan <- n
			continue
		}
		concurrentChan <- n
	}

	// close channel to terminate the workers
	close(concurrentChan)
	close(serialChan)

	return wg
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

	linksCopy := map[int]*types.Link{}
	for k, v := range c.Links {
		linksCopy[k] = v
	}
	for {
		if len(linksCopy) == 0 {
			break
		}
		for k, link := range linksCopy {
			if link.A.Node.DeploymentStatus == "created" && link.B.Node.DeploymentStatus == "created" {
				linksChan <- link
				delete(linksCopy, k)
			}
		}
	}

	// close channel to terminate the workers
	close(linksChan)
	// wait for all workers to finish
	wg.Wait()
}

func (c *CLab) DeleteNodes(ctx context.Context, workers uint, deleteCandidates map[string]nodes.Node, serialNodes map[string]struct{}) {

	wg := new(sync.WaitGroup)

	concurrentChan := make(chan nodes.Node)
	serialChan := make(chan nodes.Node)

	workerFunc := func(i uint, input chan nodes.Node, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case n := <-input:
				if n == nil {
					log.Debugf("Worker %d terminating...", i)
					return
				}
				err := n.Delete(ctx)
				if err != nil {
					log.Errorf("could not remove container %q: %v", n.Config().LongName, err)
				}
			case <-ctx.Done():
				return
			}
		}
	}

	// start concurrent workers
	wg.Add(int(workers))
	for i := uint(0); i < workers; i++ {
		go workerFunc(i, concurrentChan, wg)
	}

	// start the serial worker
	if len(serialNodes) > 0 {
		wg.Add(1)
		go workerFunc(workers, serialChan, wg)
	}

	// send nodes to workers
	for _, n := range c.Nodes {
		if _, ok := serialNodes[n.Config().LongName]; ok {
			serialChan <- n
			continue
		}
		concurrentChan <- n
	}

	// close channel to terminate the workers
	close(concurrentChan)
	close(serialChan)

	wg.Wait()

}

func (c *CLab) ListContainers(ctx context.Context, labels []*types.GenericFilter) ([]types.GenericContainer, error) {
	var containers []types.GenericContainer

	for _, r := range c.Runtimes {
		ctrs, err := r.ListContainers(ctx, labels)
		if err != nil {
			return containers, fmt.Errorf("could not list containers: %v", err)
		}
		containers = append(containers, ctrs...)
	}
	return containers, nil
}

func (c *CLab) GetNodeRuntime(query string) (runtime.ContainerRuntime, error) {
	shortName, err := getShortName(c.Config.Name, query)
	if err != nil {
		return nil, err
	}

	if node, ok := c.Nodes[shortName]; ok {
		return node.GetRuntime(), nil
	}

	return nil, fmt.Errorf("could not find a container matching name %q", query)
}

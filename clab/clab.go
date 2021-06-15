// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	log "github.com/sirupsen/logrus"
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
	Nodes    map[string]*types.NodeConfig
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
		if rInit, ok := runtime.ContainerRuntimes[name]; ok {
			c.Runtime = rInit()
			c.Runtime.Init(
				runtime.WithConfig(&runtime.RuntimeConfig{
					Timeout: dur,
					Debug:   d,
				}),
				runtime.WithMgmtNet(c.Config.Mgmt),
			)
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
			Mgmt: new(types.MgmtNet),
		},
		TopoFile: new(TopoFile),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]*types.NodeConfig),
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

func (c *CLab) CreateNode(ctx context.Context, node *types.NodeConfig, certs *Certificates) error {
	if certs != nil {
		c.m.Lock()
		node.TLSCert = string(certs.Cert)
		node.TLSKey = string(certs.Key)
		c.m.Unlock()
	}
	err := c.CreateNodeDirStructure(node)
	if err != nil {
		return err
	}
	return c.Runtime.CreateContainer(ctx, node)
}

// ExecPostDeployTasks executes tasks that some nodes might require to boot properly after start
func (c *CLab) ExecPostDeployTasks(ctx context.Context, node *types.NodeConfig, lworkers uint) error {
	switch node.Kind {
	case "ceos":
		log.Debugf("Running postdeploy actions for Arista cEOS '%s' node", node.ShortName)
		return ceosPostDeploy(ctx, c, node, lworkers)
	case "crpd":
		_, _, err := c.Runtime.Exec(ctx, node.ContainerID, []string{"service ssh restart"})
		if err != nil {
			return err
		}

	case "linux":
		log.Debugf("Running postdeploy actions for Linux '%s' node", node.ShortName)
		return disableTxOffload(node)

	case "sonic-vs":
		log.Debugf("Running postdeploy actions for sonic-vs '%s' node", node.ShortName)
		// TODO: change this calls to c.ExecNotWait
		// exec `supervisord` to start sonic services
		_, _, err := c.Runtime.Exec(ctx, node.ContainerID, []string{"supervisord"})
		if err != nil {
			return err
		}

		_, _, err = c.Runtime.Exec(ctx, node.ContainerID, []string{"/usr/lib/frr/bgpd"})
		if err != nil {
			return err
		}

	case "mysocketio":
		log.Debugf("Running postdeploy actions for mysocketio '%s' node", node.ShortName)
		err := disableTxOffload(node)
		if err != nil {
			return fmt.Errorf("failed to disable tx checksum offload for mysocketio kind: %v", err)
		}

		log.Infof("Creating mysocketio tunnels...")
		err = createMysocketTunnels(ctx, c, node)
		return err
	}
	return nil
}

func (c *CLab) CreateNodes(ctx context.Context, workers uint) {
	wg := new(sync.WaitGroup)
	wg.Add(int(workers))
	nodesChan := make(chan *types.NodeConfig)
	// start workers
	for i := uint(0); i < workers; i++ {
		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case node := <-nodesChan:
					if node == nil {
						log.Debugf("Worker %d terminating...", i)
						return
					}
					log.Debugf("Worker %d received node: %+v", i, node)
					if node.Kind == "bridge" || node.Kind == "ovs-bridge" {
						return
					}

					var nodeCerts *Certificates
					var certTpl *template.Template
					if node.Kind == "srl" {
						var err error
						nodeCerts, err = c.RetrieveNodeCertData(node)
						// if not available on disk, create cert in next step
						if err != nil {
							// create CERT
							certTpl, err = template.New("node-cert").Parse(nodeCSRTempl)
							if err != nil {
								log.Errorf("failed to parse Node CSR Template: %v", err)
							}
							certInput := CertInput{
								Name:     node.ShortName,
								LongName: node.LongName,
								Fqdn:     node.Fqdn,
								Prefix:   c.Config.Name,
							}
							nodeCerts, err = c.GenerateCert(
								path.Join(c.Dir.LabCARoot, "root-ca.pem"),
								path.Join(c.Dir.LabCARoot, "root-ca-key.pem"),
								certTpl,
								certInput,
								path.Join(c.Dir.LabCA, certInput.Name),
							)
							if err != nil {
								log.Errorf("failed to generate certificates for node %s: %v", node.ShortName, err)
							}
							log.Debugf("%s CSR: %s", node.ShortName, string(nodeCerts.Csr))
							log.Debugf("%s Cert: %s", node.ShortName, string(nodeCerts.Cert))
							log.Debugf("%s Key: %s", node.ShortName, string(nodeCerts.Key))
						}
					}
					err := c.CreateNode(ctx, node, nodeCerts)
					if err != nil {
						log.Errorf("failed to create node %s: %v", node.ShortName, err)
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
					name := cont.ID
					if len(cont.Names) > 0 {
						name = strings.TrimLeft(cont.Names[0], "/")
					}
					err := c.Runtime.DeleteContainer(ctx, name)
					if err != nil {
						log.Errorf("could not remove container '%s': %v", name, err)
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

func disableTxOffload(n *types.NodeConfig) error {
	// skip this if node runs in host mode
	if strings.ToLower(n.NetworkMode) == "host" {
		return nil
	}
	// disable tx checksum offload for linux containers on eth0 interfaces
	nodeNS, err := ns.GetNS(n.NSPath)
	if err != nil {
		return err
	}
	err = nodeNS.Do(func(_ ns.NetNS) error {
		// disabling offload on lo0 interface
		err := utils.EthtoolTXOff("eth0")
		if err != nil {
			log.Infof("Failed to disable TX checksum offload for 'eth0' interface for Linux '%s' node: %v", n.ShortName, err)
		}
		return err
	})
	return err
}

func StringInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

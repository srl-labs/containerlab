package clab

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// var debug bool

type CLab struct {
	Config       *Config
	TopoFile     *TopoFile
	m            *sync.RWMutex
	Nodes        map[string]*Node
	Links        map[int]*Link
	DockerClient *docker.Client
	Dir          *cLabDirectory

	debug            bool
	timeout          time.Duration
	gracefulShutdown bool
}

type cLabDirectory struct {
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

func WithEnvDockerClient() ClabOption {
	return func(c *CLab) {
		var err error
		c.DockerClient, err = docker.NewEnvClient()
		if err != nil {
			log.Fatalf("failed to create docker client: %v", err)
		}
	}
}

func WithTopoFile(file string) ClabOption {
	return func(c *CLab) {
		if file == "" {
			return
		}
		if err := c.GetTopology(file); err != nil {
			log.Fatalf("failed to read topology file: %v", err)
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
		Config:   new(Config),
		TopoFile: new(TopoFile),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *CLab) CreateNode(ctx context.Context, node *Node, certs *certificates) error {
	c.m.Lock()
	node.TLSCert = string(certs.Cert)
	node.TLSKey = string(certs.Key)
	c.m.Unlock()
	err := c.CreateNodeDirStructure(node)
	if err != nil {
		return err
	}
	return c.CreateContainer(ctx, node)
}

// ExecPostDeployTasks executes tasks that some nodes might require to boot properly after start
func (c *CLab) ExecPostDeployTasks(ctx context.Context, node *Node, lworkers uint) error {
	switch node.Kind {
	case "ceos":
		log.Debugf("Running postdeploy actions for Arista cEOS '%s' node", node.ShortName)
		// regenerate ceos config since it is now known which IP address docker assigned to this container
		err := node.generateConfig(node.ResConfig)
		if err != nil {
			return err
		}
		log.Infof("Restarting '%s' node", node.ShortName)
		// force stopping and start is faster than ContainerRestart
		var timeout time.Duration = 1
		err = c.DockerClient.ContainerStop(ctx, node.ContainerID, &timeout)
		if err != nil {
			return err
		}
		// remove the netns symlink created during original start
		// we will re-symlink it later
		if err := deleteNetnsSymlink(node.LongName); err != nil {
			return err
		}
		err = c.DockerClient.ContainerStart(ctx, node.ContainerID, types.ContainerStartOptions{})
		if err != nil {
			return err
		}
		// since container has been restarted, we need to get its new NSPath and link netns
		cont, err := c.DockerClient.ContainerInspect(ctx, node.ContainerID)
		if err != nil {
			return err
		}
		log.Debugf("node %s new pid %v", node.LongName, cont.State.Pid)
		node.NSPath = "/proc/" + strconv.Itoa(cont.State.Pid) + "/ns/net"
		err = linkContainerNS(node.NSPath, node.LongName)
		if err != nil {
			return err
		}
		// now its time to create the links which has one end in ceos node
		wg := new(sync.WaitGroup)
		wg.Add(int(lworkers))
		linksChan := make(chan *Link)
		c.CreateLinks(ctx, lworkers, linksChan, wg)
		for _, link := range c.Links {
			if link.A.Node.Kind == "ceos" || link.B.Node.Kind == "ceos" {
				linksChan <- link
			}
		}
		// close channel to terminate the workers
		close(linksChan)
		// wait for all workers to finish
		wg.Wait()
	case "crpd":
		// exec `service ssh restart` to start ssh service and take into account mounted sshd_config
		execConfig := types.ExecConfig{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: strings.Fields("service ssh restart")}
		respID, err := c.DockerClient.ContainerExecCreate(context.Background(), node.ContainerID, execConfig)
		if err != nil {
			return err
		}
		_, err = c.DockerClient.ContainerExecAttach(context.Background(), respID.ID, execConfig)
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
		execConfig := types.ExecConfig{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: strings.Fields("supervisord")}
		respID, err := c.DockerClient.ContainerExecCreate(context.Background(), node.ContainerID, execConfig)
		if err != nil {
			return err
		}
		_, err = c.DockerClient.ContainerExecAttach(context.Background(), respID.ID, execConfig)
		if err != nil {
			return err
		}
		// exec `/usr/lib/frr/bgpd` to start BGP service
		execConfig = types.ExecConfig{Tty: false, AttachStdout: false, AttachStderr: false, Cmd: strings.Fields("/usr/lib/frr/bgpd")}
		respID, err = c.DockerClient.ContainerExecCreate(context.Background(), node.ContainerID, execConfig)
		if err != nil {
			return err
		}
		_, err = c.DockerClient.ContainerExecAttach(context.Background(), respID.ID, execConfig)
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

// CreateLinks creates links that are passed to it over linksChan using the number of workers
func (c *CLab) CreateLinks(ctx context.Context, workers uint, linksChan chan *Link, wg *sync.WaitGroup) {
	log.Debug("creating links...")
	// wire the links between the nodes based on cabling plan
	for i := uint(0); i < workers; i++ {
		go func(i uint) {
			defer wg.Done()
			for {
				select {
				case link := <-linksChan:
					if link == nil {
						log.Debugf("Worker %d terminating...", i)
						return
					}
					log.Debugf("Worker %d received link: %+v", i, link)
					if err := c.CreateVirtualWiring(link); err != nil {
						log.Error(err)
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
}

func disableTxOffload(n *Node) error {
	// disable tx checksum offload for linux containers on eth0 interfaces
	nodeNS, err := ns.GetNS(n.NSPath)
	if err != nil {
		return err
	}
	err = nodeNS.Do(func(_ ns.NetNS) error {
		// disabling offload on lo0 interface
		err := EthtoolTXOff("eth0")
		if err != nil {
			log.Infof("Failed to disable TX checksum offload for 'eth0' interface for Linux '%s' node: %v", n.ShortName, err)
		}
		return err
	})
	return err
}

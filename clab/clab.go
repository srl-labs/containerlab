package clab

import (
	"context"
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
func (c *CLab) ExecPostDeployTasks(ctx context.Context, node *Node) error {
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
		err = c.DockerClient.ContainerStart(ctx, node.ContainerID, types.ContainerStartOptions{})
		if err != nil {
			return err
		}
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
		// disable tx checksum offload for linux containers on lo and eth0 interfaces
		nodeNS, err := ns.GetNS(node.NSPath)
		if err != nil {
			return err
		}
		err = nodeNS.Do(func(_ ns.NetNS) error {
			// disabling offload on lo0 interface
			err = EthtoolTXOff("eth0")
			if err != nil {
				log.Infof("Failed to disable TX checksum offload for 'eth0' interface for Linux '%s' node: %v", node.ShortName, err)
			}
			return nil
		})
		return err

	}
	return nil
}

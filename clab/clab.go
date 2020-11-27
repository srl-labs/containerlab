package clab

import (
	"context"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// var debug bool

type cLab struct {
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

type ClabOption func(c *cLab)

func WithDebug(d bool) ClabOption {
	return func(c *cLab) {
		c.debug = d
	}
}

func WithTimeout(dur time.Duration) ClabOption {
	return func(c *cLab) {
		c.timeout = dur
	}
}

func WithEnvDockerClient() ClabOption {
	return func(c *cLab) {
		var err error
		c.DockerClient, err = docker.NewEnvClient()
		if err != nil {
			log.Fatalf("failed to create docker client: %v", err)
		}
	}
}

func WithTopoFile(file string) ClabOption {
	return func(c *cLab) {
		if file == "" {
			return
		}
		if err := c.GetTopology(file); err != nil {
			log.Fatalf("failed to read topology file: %v", err)
		}
	}
}

func WithGracefulShutdown(gracefulShutdown bool) ClabOption {
	return func(c *cLab) {
		c.gracefulShutdown = gracefulShutdown
	}
}

// NewContainerLab function defines a new container lab
func NewContainerLab(opts ...ClabOption) *cLab {
	c := &cLab{
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

func (c *cLab) CreateNode(ctx context.Context, node *Node, certs *certificates) error {
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
func (c *cLab) ExecPostDeployTasks(ctx context.Context, node *Node) error {
	switch node.Kind {
	case "ceos":
		log.Infof("Running postdeploy actions for '%s' node", node.ShortName)
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
	}
	return nil
}

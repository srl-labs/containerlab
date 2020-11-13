package clab

import (
	"context"
	"sync"
	"time"

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

	debug   bool
	timeout time.Duration
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

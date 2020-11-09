package clab

import (
	"context"
	"sync"
	"time"

	docker "github.com/docker/docker/client"
)

// var debug bool

type cLab struct {
	Conf         *Conf
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

// NewContainerLab function defines a new container lab
func NewContainerLab(d bool) *cLab {
	return &cLab{
		Conf:     new(Conf),
		TopoFile: new(TopoFile),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
		debug:    d,
	}
}

// Init creates a docker client and adds it to containerlab structure
func (c *cLab) Init(timeout time.Duration) (err error) {
	c.DockerClient, err = docker.NewEnvClient()
	c.timeout = timeout
	return
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

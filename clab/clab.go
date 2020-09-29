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
	FileInfo     *File
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
		FileInfo: new(File),
		m:        new(sync.RWMutex),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
		debug:    d,
	}
}

func (c *cLab) Init(timeout time.Duration) (err error) {
	c.DockerClient, err = docker.NewEnvClient()
	c.timeout = timeout
	return
}

func (c *cLab) CreateNode(ctx context.Context, node *Node, certs *certificates) error {
	c.m.Lock()
	defer c.m.Unlock()
	node.TLSCert = string(certs.Cert)
	node.TLSKey = string(certs.Key)
	err := c.CreateNodeDirStructure(node)
	if err != nil {
		return err
	}
	return c.CreateContainer(ctx, node)
}

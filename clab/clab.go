package clab

import (
	"sync"
	"time"

	docker "github.com/docker/engine/client"
)

// var debug bool

type cLab struct {
	Conf         *Conf
	FileInfo     *File
	m            *sync.Mutex
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
		m:        new(sync.Mutex),
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

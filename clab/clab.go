package clab

import (
	docker "github.com/docker/engine/client"
)

// var debug bool

type cLab struct {
	Conf         *conf
	FileInfo     *File
	Nodes        map[string]*Node
	Links        map[int]*Link
	DockerClient *docker.Client
	Dir          *cLabDirectory

	debug bool
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
		Conf:     new(conf),
		FileInfo: new(File),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
		debug:    d,
	}
}

func (c *cLab) Init() (err error) {
	c.DockerClient, err = docker.NewEnvClient()
	return
}

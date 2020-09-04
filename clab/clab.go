package clab

import (
	docker "github.com/docker/engine/client"
)

var debug bool

type cLab struct {
	Conf         *conf
	FileInfo     *File
	Nodes        map[string]*Node
	Links        map[int]*Link
	DockerClient *docker.Client
	Dir          *cLabDirectory
}

type cLabDirectory struct {
	Lab       string
	LabCA     string
	LabCARoot string
	LabGraph  string
}

// NewContainerLab function defines a new container lab
func NewContainerLab(d bool) (*cLab, error) {
	c := &cLab{
		Conf:     new(conf),
		FileInfo: new(File),
		Nodes:    make(map[string]*Node),
		Links:    make(map[int]*Link),
	}
	debug = d
	var err error
	c.DockerClient, err = docker.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return c, nil
}

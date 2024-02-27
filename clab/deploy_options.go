package clab

import (
	"math"

	log "github.com/sirupsen/logrus"
	"github.com/tklauser/numcpus"
)

type DeployOptions struct {
	reconfigure    bool
	skipPostDeploy bool
	graph          bool
	maxWorkers     uint
	exportTemplate string
}

func NewDeployOptions(maxWorkers uint) (*DeployOptions, error) {
	d := &DeployOptions{
		maxWorkers: math.MaxUint,
	}
	err := d.initWorkerCount(maxWorkers)
	return d, err
}

func (d *DeployOptions) SetReconfigure(b bool) *DeployOptions {
	d.reconfigure = b
	return d
}

func (d *DeployOptions) SetSkipPostDeploy(b bool) *DeployOptions {
	d.skipPostDeploy = b
	return d
}

func (d *DeployOptions) SetGraph(b bool) *DeployOptions {
	d.graph = b
	return d
}

func (d *DeployOptions) SetMaxWorkers(i uint) *DeployOptions {
	d.maxWorkers = i
	return d
}

func (d *DeployOptions) SetExportTemplate(templatePath string) *DeployOptions {
	d.exportTemplate = templatePath
	return d
}

// countWorkers calculates the number workers used for the creation of nodes.
// If a user provided --max-workers this takes precedence.
// If maxWorkers is not set then the workers are limited by the number of available CPUs when
// number of nodes exceeds the number of available CPUs.
func (d *DeployOptions) initWorkerCount(maxWorkers uint) error {
	// init number of Workers to the number of nodes
	nodeWorkers := uint(0)

	switch {
	// if maxworkers is provided, use that value
	case maxWorkers > 0:
		nodeWorkers = maxWorkers

	// if maxWorkers is not set, limit workers number by number of available CPUs
	case maxWorkers <= 0:
		// retrieve vCPU count
		vCpus, err := numcpus.GetOnline()
		if err != nil {
			return err
		}
		nodeWorkers = uint(vCpus)
	}

	// finally set the value
	d.maxWorkers = nodeWorkers
	log.Debugf("Number of Node workers: %d", nodeWorkers)

	return nil
}

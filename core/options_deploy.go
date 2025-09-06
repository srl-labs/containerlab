package core

import (
	"github.com/charmbracelet/log"
	"github.com/tklauser/numcpus"
)

// DeployOptions represents the options for deploying a lab.
type DeployOptions struct {
	reconfigure        bool   // reconfigure indicates whether to reconfigure the lab.
	skipPostDeploy     bool   // skipPostDeploy indicates whether to skip post-deployment steps.
	graph              bool   // graph indicates whether to generate a graph of the lab.
	maxWorkers         uint   // maxWorkers is the maximum number of workers for node creation.
	exportTemplate     string // exportTemplate is the path to the export template.
	skipLabDirFileACLs bool   // skip setting the extended File ACL entries on the lab directory.
}

// NewDeployOptions creates a new DeployOptions instance with the specified maxWorkers value.
func NewDeployOptions(maxWorkers uint) (*DeployOptions, error) {
	d := &DeployOptions{}

	err := d.initWorkerCount(maxWorkers)

	return d, err
}

// SetReconfigure sets the reconfigure option and returns the updated DeployOptions instance.
func (d *DeployOptions) SetReconfigure(b bool) *DeployOptions {
	d.reconfigure = b

	return d
}

// Reconfigure returns the reconfigure option value.
func (d *DeployOptions) Reconfigure() bool {
	return d.reconfigure
}

// SetSkipPostDeploy sets the skipPostDeploy option and returns the updated DeployOptions instance.
func (d *DeployOptions) SetSkipPostDeploy(b bool) *DeployOptions {
	d.skipPostDeploy = b
	return d
}

// SetSkipLabDirFileACLs sets the skipLabDirFileACLs deployment option.
func (d *DeployOptions) SetSkipLabDirFileACLs(b bool) *DeployOptions {
	d.skipLabDirFileACLs = b
	return d
}

// SkipPostDeploy returns the skipPostDeploy option value.
func (d *DeployOptions) SkipPostDeploy() bool {
	return d.skipPostDeploy
}

// SetGraph sets the graph option and returns the updated DeployOptions instance.
func (d *DeployOptions) SetGraph(b bool) *DeployOptions {
	d.graph = b

	return d
}

// Graph returns the graph option value.
func (d *DeployOptions) Graph() bool {
	return d.graph
}

// SetMaxWorkers sets the maxWorkers option and returns the updated DeployOptions instance.
func (d *DeployOptions) SetMaxWorkers(i uint) *DeployOptions {
	d.maxWorkers = i

	return d
}

// MaxWorkers returns the maxWorkers option value.
func (d *DeployOptions) MaxWorkers() uint {
	return d.maxWorkers
}

// SetExportTemplate sets the exportTemplate option and returns the updated DeployOptions instance.
func (d *DeployOptions) SetExportTemplate(templatePath string) *DeployOptions {
	d.exportTemplate = templatePath

	return d
}

// ExportTemplate returns the exportTemplate option value.
func (d *DeployOptions) ExportTemplate() string {
	return d.exportTemplate
}

// initWorkerCount calculates the number of workers used for node creation.
// If maxWorkers is provided, it takes precedence.
// If maxWorkers is not set, the number of workers is limited by the number of available CPUs
// when the number of nodes exceeds the number of available CPUs.
func (d *DeployOptions) initWorkerCount(maxWorkers uint) error {
	// init number of Workers to the number of nodes
	nodeWorkers := uint(0)

	switch {
	// if maxWorkers is provided, use that value
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

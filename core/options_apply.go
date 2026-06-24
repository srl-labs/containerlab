package core

import (
	"github.com/charmbracelet/log"
	"github.com/tklauser/numcpus"
)

// ApplyOptions represents options for applying a topology file to a lab.
type ApplyOptions struct {
	dryRun         bool
	skipPostDeploy bool
	maxWorkers     uint
	exportTemplate string
}

// NewApplyOptions creates a new ApplyOptions instance.
func NewApplyOptions(maxWorkers uint) (*ApplyOptions, error) {
	o := &ApplyOptions{}

	err := o.initWorkerCount(maxWorkers)

	return o, err
}

func (o *ApplyOptions) SetDryRun(v bool) *ApplyOptions {
	o.dryRun = v
	return o
}

func (o *ApplyOptions) SetSkipPostDeploy(v bool) *ApplyOptions {
	o.skipPostDeploy = v
	return o
}

func (o *ApplyOptions) SetExportTemplate(v string) *ApplyOptions {
	o.exportTemplate = v
	return o
}

func (o *ApplyOptions) initWorkerCount(maxWorkers uint) error {
	switch {
	case maxWorkers > 0:
		o.maxWorkers = maxWorkers
	default:
		vCpus, err := numcpus.GetOnline()
		if err != nil {
			return err
		}

		o.maxWorkers = uint(vCpus)
	}

	log.Debugf("Number of apply workers: %d", o.maxWorkers)

	return nil
}

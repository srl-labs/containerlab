package clab

import "github.com/srl-labs/containerlab/types"

type ExecOptions struct {
	filters []*types.GenericFilter
}

func NewExecOptions(filters []*types.GenericFilter) *ExecOptions {
	return &ExecOptions{
		filters: filters,
	}
}

func (e *ExecOptions) AddFilters(f ...*types.GenericFilter) {
	e.filters = append(e.filters, f...)
}

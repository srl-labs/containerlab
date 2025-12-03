package core

import (
	"strings"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// ListOption is a type used for functional options for the Clab List method.
type ListOption func(o *ListOptions)

// ListOptions represents the options for listing containers.
type ListOptions struct {
	containerName   string
	labName         string
	nodeName        string
	toolType        string
	clabLabelExists bool
	cliArgs         []string
	filters         []*clabtypes.GenericFilter
}

// NewListOptions returns a new list options object.
func NewListOptions() *ListOptions {
	return &ListOptions{}
}

// ToFilters converts the list options to a slice of generic filters.
func (o *ListOptions) ToFilters() []*clabtypes.GenericFilter {
	var filters []*clabtypes.GenericFilter

	if o.containerName != "" {
		filters = append(
			filters,
			&clabtypes.GenericFilter{
				FilterType: "name",
				Match:      o.containerName,
			},
		)
	}

	if o.labName != "" {
		filters = append(
			filters,
			&clabtypes.GenericFilter{
				FilterType: "label",
				Field:      clabconstants.Containerlab,
				Operator:   "=",
				Match:      o.labName,
			},
		)
	}

	if o.nodeName != "" {
		filters = append(
			filters,
			&clabtypes.GenericFilter{
				FilterType: "label",
				Field:      clabconstants.LongName,
				Operator:   "=",
				Match:      o.nodeName,
			},
		)
	}

	if o.toolType != "" {
		filters = append(
			filters,
			&clabtypes.GenericFilter{
				FilterType: "label",
				Field:      clabconstants.ToolType,
				Operator:   "=",
				Match:      o.toolType,
			},
		)
	}

	if o.clabLabelExists {
		filters = append(
			filters,
			&clabtypes.GenericFilter{
				FilterType: "label",
				Field:      clabconstants.Containerlab,
				Operator:   "exists",
			},
		)
	}

	for _, cliArg := range o.cliArgs {
		if strings.Contains(cliArg, "=") {
			cliArgParts := strings.Split(cliArg, "=")

			if len(cliArgParts) != 2 { //nolint: mnd
				// silently ignoring for safety
				continue
			}

			filters = append(
				filters,
				&clabtypes.GenericFilter{
					FilterType: "label",
					Field:      cliArgParts[0],
					Operator:   "=",
					Match:      cliArgParts[1],
				},
			)
		} else {
			filters = append(
				filters,
				&clabtypes.GenericFilter{
					FilterType: "label",
					Field:      strings.TrimSpace(cliArg),
					Operator:   "exists",
				},
			)
		}
	}

	filters = append(filters, o.filters...)

	return filters
}

// WithListContainerName filters the list operation to the given container name.
func WithListContainerName(
	s string,
) ListOption {
	return func(o *ListOptions) {
		o.containerName = s
	}
}

// WithListLabName filters the list operation to the given lab name.
func WithListLabName(
	s string,
) ListOption {
	return func(o *ListOptions) {
		o.labName = s
	}
}

// WithListNodeName filters the list operation to the given node name.
func WithListNodeName(
	s string,
) ListOption {
	return func(o *ListOptions) {
		o.nodeName = s
	}
}

// WithListToolType filters the list operation to the tool type name.
func WithListToolType(
	s string,
) ListOption {
	return func(o *ListOptions) {
		o.toolType = s
	}
}

// WithListclabLabelExists filters the list to any containers that include a containerlab
// label.
func WithListclabLabelExists() ListOption {
	return func(o *ListOptions) {
		o.clabLabelExists = true
	}
}

// WithListFromCliArgs filters the list based on a string slice of cli args, transforming those args
// into proper filters in the ToFilters method.
func WithListFromCliArgs(ss []string) ListOption {
	return func(o *ListOptions) {
		o.cliArgs = ss
	}
}

// WithListFilter adds user created normal filters to the filter set.
func WithListFilter(fs []*clabtypes.GenericFilter) ListOption {
	return func(o *ListOptions) {
		o.filters = fs
	}
}

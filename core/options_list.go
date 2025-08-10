package core

import (
	"strings"

	containerlablabels "github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/types"
)

// ListOption is a type used for functional options for the Clab List method.
type ListOption func(o *ListOptions)

// ListOptions represents the options for listing containers.
type ListOptions struct {
	labName                 string
	nodeName                string
	toolType                string
	containerlabLabelExists bool
	cliArgs                 []string
}

// NewListOptions returns a new list options object.
func NewListOptions() *ListOptions {
	return &ListOptions{}
}

// ToFilters converts the list options to a slice of generic filters.
func (o *ListOptions) ToFilters() []*types.GenericFilter {
	var filters []*types.GenericFilter

	if o.labName != "" {
		filters = append(
			filters,
			&types.GenericFilter{
				FilterType: "label",
				Field:      containerlablabels.Containerlab,
				Operator:   "=",
				Match:      o.labName,
			},
		)
	}

	if o.nodeName != "" {
		filters = append(
			filters,
			&types.GenericFilter{
				FilterType: "label",
				Field:      containerlablabels.LongName,
				Operator:   "=",
				Match:      o.nodeName,
			},
		)
	}

	if o.toolType != "" {
		filters = append(
			filters,
			&types.GenericFilter{
				FilterType: "label",
				Field:      containerlablabels.ToolType,
				Operator:   "=",
				Match:      o.toolType,
			},
		)
	}

	if o.containerlabLabelExists {
		filters = append(
			filters,
			&types.GenericFilter{
				FilterType: "label",
				Field:      containerlablabels.Containerlab,
				Operator:   "exists",
			},
		)
	}

	for _, cliArg := range o.cliArgs {
		if strings.Contains(cliArg, "=") {
			cliArgParts := strings.Split(cliArg, "=")

			if len(cliArgParts) != 2 {
				// silently ignoring for safety
				continue
			}

			filters = append(
				filters,
				&types.GenericFilter{
					FilterType: "label",
					Field:      cliArgParts[0],
					Operator:   "=",
					Match:      cliArgParts[1],
				},
			)
		} else {
			filters = append(
				filters,
				&types.GenericFilter{
					FilterType: "label",
					Field:      strings.TrimSpace(cliArg),
					Operator:   "exists",
				},
			)
		}
	}

	return filters
}

// WithLabName filters the list operation to the given lab name.
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

// WithListContainerlabLabelExists filters the list to any containers that include a containerlab
// label.
func WithListContainerlabLabelExists() ListOption {
	return func(o *ListOptions) {
		o.containerlabLabelExists = true
	}
}

// WithListFromCliArgs filters the list based on a string slice of cli args, transforming those args
// into proper filters in the ToFilters method.
func WithListFromCliArgs(ss []string) ListOption {
	return func(o *ListOptions) {
		o.cliArgs = ss
	}
}

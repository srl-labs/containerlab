package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
)

func validateCmd(o *Options) (*cobra.Command, error) {
	format := clabconstants.FormatTable

	c := &cobra.Command{
		Use:   "validate",
		Short: "validate a topology file",
		Long: "parse and validate a topology definition file without deploying it" +
			"\nreference: https://containerlab.dev/cmd/validate/",
		Aliases:      []string{"val"},
		SilenceUsage: true,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return validateFn(cobraCmd.Context(), o, format)
		},
	}

	c.Flags().StringVarP(&format, "format", "f", format, "output format. One of [table, json]")

	return c, nil
}

// validateFn parses the topology (NewContainerLab runs all schema/node checks)
// and resolves links, reporting any error without touching the runtime state.
func validateFn(ctx context.Context, o *Options, format string) error {
	if format != clabconstants.FormatTable && format != clabconstants.FormatJSON {
		return fmt.Errorf("output format %q is not supported, use one of [table, json]", format)
	}

	var errs []error

	name := ""
	nodes := 0
	links := 0

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		errs = append(errs, err)
	}

	switch {
	case c == nil:
	case err == nil && (c.Config.Name == "" || len(c.Nodes) == 0):
		return fmt.Errorf("topology file %q defines no name or nodes. likely an empty file", c.TopoPaths.TopologyFilenameBase())
	default:
		name = c.Config.Name

		if err := c.ResolveLinks(); err != nil {
			errs = append(errs, err)
		}

		if err := c.ValidateTopology(ctx); err != nil {
			errs = append(errs, err)
		}

		nodes = len(c.Nodes)
		links = len(c.Links)
	}

	issues := flattenErrors(errs)

	if format == clabconstants.FormatJSON {
		messages := make([]string, 0, len(issues))
		for _, issue := range issues {
			messages = append(messages, issue.Error())
		}

		out, err := json.MarshalIndent(struct {
			Name   string   `json:"name"`
			Valid  bool     `json:"valid"`
			Nodes  int      `json:"nodes"`
			Links  int      `json:"links"`
			Errors []string `json:"errors"`
		}{
			Name:   name,
			Valid:  len(issues) == 0,
			Nodes:  nodes,
			Links:  links,
			Errors: messages,
		}, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(out))
	}

	if len(issues) > 0 {
		if format == clabconstants.FormatTable {
			for _, issue := range issues {
				log.Error(issue)
			}
		}

		return fmt.Errorf("topology is invalid: %d error(s) found", len(issues))
	}

	if format == clabconstants.FormatTable {
		log.Info("Topology is valid", "name", name, "nodes", nodes, "links", links)
	}

	return nil
}

func flattenErrors(errs []error) []error {
	var flat []error

	for _, err := range errs {
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			flat = append(flat, flattenErrors(joined.Unwrap())...)

			continue
		}

		flat = append(flat, err)
	}

	return flat
}

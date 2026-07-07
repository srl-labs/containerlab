package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clabcore "github.com/srl-labs/containerlab/core"
)

func validateCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "validate",
		Short: "validate a topology file",
		Long: "parse and validate a topology definition file without deploying it" +
			"\nreference: https://containerlab.dev/cmd/validate/",
		Aliases:      []string{"val"},
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return validateFn(o)
		},
	}

	return c, nil
}

// validateFn parses the topology (NewContainerLab runs all schema/node checks)
// and resolves links, reporting any error without touching the runtime state.
func validateFn(o *Options) error {
	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	if err := c.ResolveLinks(); err != nil {
		return err
	}

	log.Info("Topology is valid", "name", c.Config.Name,
		"nodes", len(c.Nodes), "links", len(c.Links))

	return nil
}

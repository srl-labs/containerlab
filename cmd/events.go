package cmd

import (
	"github.com/spf13/cobra"
	clabevents "github.com/srl-labs/containerlab/core/events"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func eventsCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "events",
		Short: "stream lab lifecycle and interface events",
		Long: "stream container runtime events and interface updates for all running labs using the selected runtime\n" +
			"reference: https://containerlab.dev/cmd/events/",
		Aliases: []string{"ev"},
		PreRunE: func(*cobra.Command, []string) error {
			return clabutils.CheckAndGetRootPrivs()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return eventsFn(cmd, o)
		},
	}

	c.Flags().StringVarP(
		&o.Events.Format,
		"format",
		"f",
		o.Events.Format,
		"output format. One of [plain, json]",
	)

	c.Example = `# Stream container and interface events in plain text
containerlab events

# Stream events as JSON
containerlab events --format json`

	return c, nil
}

func eventsFn(cmd *cobra.Command, o *Options) error {
	opts := clabevents.Options{
		Format:      o.Events.Format,
		Runtime:     o.Global.Runtime,
		ClabOptions: o.ToClabOptions(),
		Writer:      cmd.OutOrStdout(),
	}

	return clabevents.Stream(cmd.Context(), opts)
}

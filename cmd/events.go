package cmd

import (
	"github.com/spf13/cobra"
	clabevents "github.com/srl-labs/containerlab/clab"
)

func eventsCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "events",
		Short: "stream lab lifecycle and interface events",
		Long: "stream docker runtime events as well as container interface updates for all running labs\n" +
			"reference: https://containerlab.dev/cmd/events/",
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

	return c, nil
}

func eventsFn(cmd *cobra.Command, o *Options) error {
	opts := clabevents.EventsOptions{
		Format:      o.Events.Format,
		Runtime:     o.Global.Runtime,
		ClabOptions: o.ToClabOptions(),
	}

	return clabevents.Events(cmd.Context(), opts)
}

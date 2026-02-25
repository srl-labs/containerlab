package cmd

import (
	"time"

	"github.com/spf13/cobra"
	clabevents "github.com/srl-labs/containerlab/core/events"
	clabutils "github.com/srl-labs/containerlab/utils"
)

func eventsCmd(o *Options) (*cobra.Command, error) {
	trafficProtocols := false
	trafficInterval := 5 * time.Second

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
			return eventsFn(cmd, o, trafficProtocols, trafficInterval)
		},
	}

	c.Flags().StringVarP(
		&o.Events.Format,
		"format",
		"f",
		o.Events.Format,
		"output format. One of [plain, json]",
	)

	c.Flags().BoolVarP(
		&o.Events.IncludeInitialState,
		"initial-state",
		"i",
		o.Events.IncludeInitialState,
		"emit the current container and interface states before streaming new events",
	)

	c.Flags().BoolVar(
		&o.Events.IncludeInterfaceStats,
		"interface-stats",
		o.Events.IncludeInterfaceStats,
		"include interface statistics updates when streaming events",
	)

	c.Flags().DurationVar(
		&o.Events.StatsInterval,
		"interface-stats-interval",
		o.Events.StatsInterval,
		"interval between interface statistics samples (requires --interface-stats)",
	)

	c.Flags().BoolVar(
		&trafficProtocols,
		"traffic-protocols",
		trafficProtocols,
		"include tc/eBPF protocol traffic samples",
	)

	c.Flags().DurationVar(
		&trafficInterval,
		"traffic-interval",
		trafficInterval,
		"sample window for protocol traffic counters (requires --traffic-protocols)",
	)

	c.Example = `# Stream container and interface events in plain text
containerlab events

# Stream events as JSON
containerlab events --format json

# Stream protocol traffic samples every 5 seconds
containerlab events --traffic-protocols --traffic-interval 5s`

	return c, nil
}

func eventsFn(cmd *cobra.Command, o *Options, trafficProtocols bool, trafficInterval time.Duration) error {
	opts := clabevents.Options{
		Format:                o.Events.Format,
		Runtime:               o.Global.Runtime,
		IncludeInitialState:   o.Events.IncludeInitialState,
		IncludeInterfaceStats: o.Events.IncludeInterfaceStats,
		StatsInterval:         o.Events.StatsInterval,
		IncludeTrafficTypes:   trafficProtocols,
		TrafficInterval:       trafficInterval,
		ClabOptions:           o.ToClabOptions(),
		Writer:                cmd.OutOrStdout(),
	}

	return clabevents.Stream(cmd.Context(), opts)
}

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabutils "github.com/srl-labs/containerlab/utils"
)

// CaptureOptions are the flags of the capture command.
type CaptureOptions struct {
	Targets     []string
	Duration    time.Duration
	PacketLimit int
	SnapLength  int
	Write       string
	List        bool
	Format      string
}

func captureCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "capture",
		Short: "capture packets on lab link interfaces",
		Long: "capture packets on lab link interfaces of one or more nodes into one pcapng " +
			"stream\nreference: https://containerlab.dev/cmd/capture/",
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if commandSkipsRoot(o.Global.Runtime) {
				return nil
			}

			return clabutils.CheckAndGetRootPrivs()
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return captureFn(cmd, o)
		},
	}

	c.Flags().StringArrayVar(
		&o.Capture.Targets,
		"target",
		o.Capture.Targets,
		"node and interfaces to capture, as <node>:<interface>[,<interface>...]; repeatable",
	)
	c.Flags().DurationVar(
		&o.Capture.Duration,
		"duration",
		o.Capture.Duration,
		"stop after this duration; 0 captures until interrupted",
	)
	c.Flags().IntVar(
		&o.Capture.PacketLimit,
		"packet-limit",
		o.Capture.PacketLimit,
		"stop after this many packets across all targets; 0 means no limit",
	)
	c.Flags().IntVar(
		&o.Capture.SnapLength,
		"snap-length",
		o.Capture.SnapLength,
		"bytes kept of each packet (64 to 1048576); 0 uses the runtime default",
	)
	c.Flags().StringVarP(
		&o.Capture.Write,
		"write",
		"w",
		o.Capture.Write,
		"pcapng output file; - writes to stdout",
	)
	c.Flags().BoolVar(
		&o.Capture.List,
		"list",
		o.Capture.List,
		"list the capturable interfaces instead of capturing",
	)
	c.Flags().StringVarP(
		&o.Capture.Format,
		"format",
		"f",
		o.Capture.Format,
		"output format of --list: table or json",
	)

	c.Example = `# Capture e1-1 of srl1 and e1-1 of srl2 for 30 seconds into one file
containerlab -r c9s capture -t srl.clab.yml --target srl1:e1-1 --target srl2:e1-1 \
  --duration 30s -w srl.pcapng

# Capture two interfaces of one node and watch them live in Wireshark
containerlab -r c9s capture --name srl --target srl1:e1-1,e1-2 -w - | wireshark -k -i -

# List the interfaces capture accepts
containerlab -r c9s capture --name srl --list -f json`

	return c, nil
}

func captureFn(cmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		return fmt.Errorf("provide either a lab name (--name) or a topology file path (--topo)")
	}

	if o.Capture.List {
		return captureListFn(cmd, o)
	}

	targets, err := parseCaptureTargets(o.Capture.Targets)
	if err != nil {
		return err
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if o.Capture.Write != "" && o.Capture.Write != "-" {
		file, err := os.Create(o.Capture.Write)
		if err != nil {
			return fmt.Errorf("failed to create the capture file: %w", err)
		}
		defer file.Close()
		output = file
	}

	// An interrupt ends the capture; the command still finishes the file and exits cleanly.
	o.Global.FinishOnCancel.Store(true)
	defer o.Global.FinishOnCancel.Store(false)

	err = c.Capture(cmd.Context(), clablabruntime.CaptureRequest{
		Targets:     targets,
		Duration:    o.Capture.Duration,
		PacketLimit: o.Capture.PacketLimit,
		SnapLength:  o.Capture.SnapLength,
	}, output)
	if err != nil {
		return err
	}

	if file, ok := output.(*os.File); ok && output != cmd.OutOrStdout() {
		if err := file.Sync(); err != nil {
			return fmt.Errorf("failed to write the capture file: %w", err)
		}
	}

	return nil
}

// parseCaptureTargets parses --target values: <node>:<interface>[,<interface>...]. The node name
// ends at the first colon, so interface names may contain colons or slashes.
func parseCaptureTargets(values []string) ([]clablabruntime.CaptureTarget, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("provide at least one --target <node>:<interface>[,<interface>...]")
	}

	targets := make([]clablabruntime.CaptureTarget, 0, len(values))
	for _, value := range values {
		node, interfaces, ok := strings.Cut(strings.TrimSpace(value), ":")
		node = strings.TrimSpace(node)
		if !ok || node == "" || strings.TrimSpace(interfaces) == "" {
			return nil, fmt.Errorf("invalid --target %q: use <node>:<interface>[,<interface>...]",
				value)
		}

		for _, name := range strings.Split(interfaces, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("invalid --target %q: empty interface name", value)
			}
			targets = append(targets, clablabruntime.CaptureTarget{Node: node, Interface: name})
		}
	}

	return targets, nil
}

func captureListFn(cmd *cobra.Command, o *Options) error {
	format := o.Capture.Format
	if format == "" {
		format = clabconstants.FormatTable
	}
	if format != clabconstants.FormatTable && format != clabconstants.FormatJSON {
		return fmt.Errorf("output format %q is not supported, use 'table' or 'json'", format)
	}
	if len(o.Capture.Targets) != 0 {
		return fmt.Errorf("--list lists every running node's interfaces; it takes no --target")
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	interfaces, err := c.CaptureInterfaces(cmd.Context())
	if err != nil {
		return err
	}

	return printCaptureInterfaces(cmd.OutOrStdout(), interfaces, format)
}

func printCaptureInterfaces(
	w io.Writer,
	interfaces []clablabruntime.CaptureInterface,
	format string,
) error {
	if format == clabconstants.FormatJSON {
		if interfaces == nil {
			interfaces = []clablabruntime.CaptureInterface{}
		}
		data, err := json.MarshalIndent(interfaces, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))

		return err
	}

	table := tableWriter.NewWriter()
	table.SetOutputMirror(w)
	table.SetStyle(tableWriter.StyleRounded)
	table.Style().Format.Header = text.FormatTitle
	table.Style().Format.HeaderAlign = text.AlignCenter
	table.AppendHeader(tableWriter.Row{"Node", "Interface", "Alias", "Peer Node", "Peer Interface"})
	table.SetColumnConfigs([]tableWriter.ColumnConfig{{Number: 1, AutoMerge: true}})
	for _, intf := range interfaces {
		alias := intf.Alias
		if alias == "" {
			alias = clabconstants.NotApplicable
		}
		table.AppendRow(tableWriter.Row{
			intf.Node, intf.Interface, alias, intf.PeerNode, intf.PeerInterface,
		})
	}
	table.Render()

	return nil
}

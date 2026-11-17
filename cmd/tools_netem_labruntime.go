package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// netemLabRuntimeSelected reports whether tools netem runs with a lab runtime, which sets the
// impairments on the runtime's links instead of in local network namespaces.
func netemLabRuntimeSelected(o *Options) bool {
	return clablabruntime.IsLabRuntimeName(o.Global.Runtime)
}

func netemLab(o *Options) (*clabcore.CLab, error) {
	if o.Global.TopologyFile == "" && o.Global.TopologyName == "" {
		return nil, fmt.Errorf("--topo/-t or --name is required with lab runtime %q",
			o.Global.Runtime)
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to load topology: %w", err)
	}

	return c, nil
}

func netemSetLabRuntime(ctx context.Context, o *Options) error {
	c, err := netemLab(o)
	if err != nil {
		return err
	}

	err = c.SetLinkImpairment(ctx, clablabruntime.ImpairmentRequest{
		Node:      o.ToolsNetem.ContainerName,
		Interface: o.ToolsNetem.Interface,
		Impairment: &clablabruntime.Impairment{
			Delay:      o.ToolsNetem.Delay,
			Jitter:     o.ToolsNetem.Jitter,
			Loss:       o.ToolsNetem.Loss,
			Rate:       o.ToolsNetem.Rate,
			Corruption: o.ToolsNetem.Corruption,
		},
	})
	if err != nil {
		return err
	}

	nodes, err := c.LinkImpairments(ctx, []string{o.ToolsNetem.ContainerName})
	if err != nil {
		return err
	}

	for _, node := range nodes {
		for _, impairment := range node.Impairments {
			if impairment.Interface == o.ToolsNetem.Interface {
				printImpairments([]tableWriter.Row{impairmentTableRow(impairment)})
			}
		}
	}

	return nil
}

func netemShowLabRuntime(ctx context.Context, o *Options) error {
	c, err := netemLab(o)
	if err != nil {
		return err
	}

	var requested []string
	if o.ToolsNetem.ContainerName != "" {
		requested = []string{o.ToolsNetem.ContainerName}
	}

	nodes, err := c.LinkImpairments(ctx, requested)
	if err != nil {
		return err
	}

	if o.ToolsNetem.Format == clabconstants.FormatJSON {
		impairments := make(map[string][]clabtypes.ImpairmentData, len(nodes))
		for _, node := range nodes {
			impairments[node.Node] = node.Impairments
		}

		jsonData, err := json.MarshalIndent(impairments, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %v", err)
		}

		fmt.Println(string(jsonData))

		return nil
	}

	for _, node := range nodes {
		if len(requested) == 0 {
			fmt.Printf("\n=== Node: %s ===\n", node.Node)
		}

		rows := make([]tableWriter.Row, 0, len(node.Impairments))
		for _, impairment := range node.Impairments {
			rows = append(rows, impairmentTableRow(impairment))
		}

		printImpairments(rows)
	}

	return nil
}

func netemResetLabRuntime(ctx context.Context, o *Options) error {
	c, err := netemLab(o)
	if err != nil {
		return err
	}

	err = c.SetLinkImpairment(ctx, clablabruntime.ImpairmentRequest{
		Node:      o.ToolsNetem.ContainerName,
		Interface: o.ToolsNetem.Interface,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Reset impairments on node %q, interface %q\n",
		o.ToolsNetem.ContainerName, o.ToolsNetem.Interface)

	return nil
}

// impairmentTableRow formats an impairment like the table tools netem show prints for local
// labs.
func impairmentTableRow(impairment clabtypes.ImpairmentData) tableWriter.Row {
	return tableWriter.Row{
		impairment.Interface,
		impairment.Delay,
		impairment.Jitter,
		strconv.FormatFloat(impairment.PacketLoss, 'f', 2, 64) + "%",
		strconv.Itoa(impairment.Rate),
		strconv.FormatFloat(impairment.Corruption, 'f', 2, 64) + "%",
	}
}

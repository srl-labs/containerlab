package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabcore "github.com/srl-labs/containerlab/core"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func inspectInterfacesFn(cobraCmd *cobra.Command, o *Options) error {
	if o.Global.TopologyName == "" && o.Global.TopologyFile == "" {
		fmt.Println("provide either a lab name (--name) or a topology file path (--topo)")
		return nil
	}

	if o.Inspect.InterfacesFormat != clabconstants.FormatTable &&
		o.Inspect.InterfacesFormat != clabconstants.FormatJSON {
		return fmt.Errorf(
			"output format %v is not supported, use 'table' or 'json'",
			o.Inspect.InterfacesFormat,
		)
	}

	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return fmt.Errorf("could not parse the topology file: %v", err)
	}

	var labNameFilterLabel string

	switch {
	case o.Global.TopologyName != "":
		labNameFilterLabel = o.Global.TopologyName
	case c.Config.Name != "":
		labNameFilterLabel = c.Config.Name
	default:
		return fmt.Errorf("could not find topology")
	}

	listOpts := []clabcore.ListOption{
		clabcore.WithListLabName(labNameFilterLabel),
	}

	if o.Inspect.InterfacesNode != "" {
		listOpts = append(
			listOpts,
			clabcore.WithListNodeName(o.Inspect.InterfacesNode),
		)
	}

	containers, err := c.ListContainers(cobraCmd.Context(), listOpts...)
	if err != nil {
		return fmt.Errorf("failed to list containers: %s", err)
	}

	if len(containers) == 0 {
		log.Info("no containers found")
		return nil
	}

	containerInterfaces, err := c.ListContainersInterfaces(cobraCmd.Context(), containers)
	if err != nil {
		return fmt.Errorf("failed to list container interfaces: %s", err)
	}

	return printContainerInterfaces(containerInterfaces, o.Inspect.InterfacesFormat)
}

func interfacesToTableData(contInterfaces []*clabtypes.ContainerInterfaces) *[]tableWriter.Row {
	tabData := make([]tableWriter.Row, 0)

	for _, container := range contInterfaces {
		for _, iface := range container.Interfaces {
			tabRow := tableWriter.Row{}
			ifaceAlias := clabconstants.NotApplicable

			if iface.InterfaceAlias != "" {
				ifaceAlias = iface.InterfaceAlias
			}

			tabRow = append(tabRow,
				container.ContainerName,
				iface.InterfaceName,
				ifaceAlias,
				iface.InterfaceMAC,
				iface.InterfaceIndex,
				iface.InterfaceMTU,
				iface.InterfaceType,
				iface.InterfaceState,
			)

			tabData = append(tabData, tabRow)
		}
	}

	return &tabData
}

func printContainerInterfaces(
	containerInterfaces []*clabtypes.ContainerInterfaces,
	format string,
) error {
	switch format {
	case clabconstants.FormatJSON:
		b, err := json.MarshalIndent(containerInterfaces, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal container details: %v", err)
		}

		fmt.Println(string(b))

		return nil
	case "table":
		table := tableWriter.NewWriter()
		table.SetOutputMirror(os.Stdout)
		table.SetStyle(tableWriter.StyleRounded)
		table.Style().Format.Header = text.FormatTitle
		table.Style().Format.HeaderAlign = text.AlignCenter
		table.Style().Options.SeparateRows = true
		table.Style().Color = tableWriter.ColorOptions{
			Header: text.Colors{text.Bold},
		}

		header := tableWriter.Row{
			"Container Name",
			"Name",
			"Alias",
			"MAC",
			"Index",
			"MTU",
			"Type",
			"State",
		}

		table.AppendHeader(append(tableWriter.Row{}, header...))

		// Merge container names and colorize State column
		table.SetColumnConfigs([]tableWriter.ColumnConfig{
			{Number: 1, AutoMerge: true},
			{
				Name: "State",
				Transformer: func(val interface{}) string {
					state := strings.ToLower(val.(string))
					switch {
					case state == "up":
						return text.Colors{text.FgGreen}.Sprint(state)
					case strings.Contains(state, "down"):
						return text.Colors{text.FgRed}.Sprint(state)
					default:
						return text.Colors{text.FgYellow}.Sprint(state)
					}
				},
			},
		})

		tabData := interfacesToTableData(containerInterfaces)
		table.AppendRows(*tabData)

		table.Render()

		return nil
	}

	return nil
}

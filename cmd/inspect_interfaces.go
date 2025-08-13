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

	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var (
	interfacesFormat   string
	interfacesNodeName string
)

// inspectInterfacesCmd represents the inspect interfaces command.
var inspectInterfacesCmd = &cobra.Command{
	Use:     "interfaces",
	Short:   "inspect interfaces of one or multiple nodes in a lab",
	Long:    "show interfaces and their attributes in a specific deployed lab\nreference: https://containerlab.dev/cmd/inspect/interfaces/",
	Aliases: []string{"int", "intf"},
	RunE:    inspectInterfacesFn,
	PreRunE: clabutils.CheckAndGetRootPrivs,
}

func init() {
	InspectCmd.AddCommand(inspectInterfacesCmd)

	inspectInterfacesCmd.Flags().StringVarP(&interfacesFormat, "format", "f", "table", "output format. One of [table, json]")
	inspectInterfacesCmd.Flags().StringVarP(&interfacesNodeName, "node", "n", "", "node to inspect")
}

func inspectInterfacesFn(cobraCmd *cobra.Command, _ []string) error {
	if labName == "" && topoFile == "" {
		fmt.Println("provide either a lab name (--name) or a topology file path (--topo)")
		return nil
	}

	if interfacesFormat != "table" && interfacesFormat != "json" {
		return fmt.Errorf("output format %v is not supported, use 'table' or 'json'", interfacesFormat)
	}

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(timeout),
		clabcore.WithRuntime(
			runtime,
			&clabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(debug),
	}

	if topoFile != "" {
		opts = append(opts,
			clabcore.WithTopoPath(topoFile, varsFile),
			clabcore.WithNodeFilter(nodeFilter),
		)
	}

	c, err := clabcore.NewContainerLab(opts...)
	if err != nil {
		return fmt.Errorf("could not parse the topology file: %v", err)
	}

	labNameFilterLabel := ""
	if labName != "" {
		labNameFilterLabel = labName
	} else if c.Config.Name != "" {
		labNameFilterLabel = c.Config.Name
	} else {
		return fmt.Errorf("could not find topology")
	}

	listOpts := []clabcore.ListOption{
		clabcore.WithListLabName(labNameFilterLabel),
	}

	if interfacesNodeName != "" {
		listOpts = append(
			listOpts,
			clabcore.WithListNodeName(interfacesNodeName),
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

	err = printContainerInterfaces(containerInterfaces, interfacesFormat)
	return err
}

func interfacesToTableData(contInterfaces []*clabtypes.ContainerInterfaces) *[]tableWriter.Row {
	tabData := make([]tableWriter.Row, 0)
	for _, container := range contInterfaces {
		for _, iface := range container.Interfaces {
			tabRow := tableWriter.Row{}
			ifaceAlias := "N/A"
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
	case "json":
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

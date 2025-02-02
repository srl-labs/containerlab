package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	tableWriter "github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/cmd/common"
	"github.com/srl-labs/containerlab/labels"
	clabRuntime "github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"

	"github.com/vishvananda/netlink"
	netlinkNs "github.com/vishvananda/netns"
)

var (
	interfacesFormat   string
	interfacesNodeName string
)

// inspectInterfacesCmd represents the inspect interfaces command.
var inspectInterfacesCmd = &cobra.Command{
	Use:     "interfaces",
	Short:   "inspect interfaces of one or multiple nodes in a lab",
	Long:    "show interfaces and their attributes in a specific deployed lab\nreference: https://containerlab.dev/cmd/inspect-interfaces/",
	Aliases: []string{"int", "intf"},
	RunE:    inspectInterfacesFn,
	PreRunE: common.CheckAndGetRootPrivs,
}

func init() {
	InspectCmd.AddCommand(inspectInterfacesCmd)

	inspectInterfacesCmd.Flags().StringVarP(&interfacesFormat, "format", "f", "table", "output format. One of [table, json]")
	inspectInterfacesCmd.Flags().StringVarP(&interfacesNodeName, "node", "n", "", "node to inspect")
}

func inspectInterfacesFn(_ *cobra.Command, _ []string) error {
	if common.Name == "" && common.Topo == "" {
		fmt.Println("provide either a lab name (--name) or a topology file path (--topo)")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := []clab.ClabOption{
		clab.WithTimeout(common.Timeout),
		clab.WithRuntime(common.Runtime,
			&clabRuntime.RuntimeConfig{
				Debug:            common.Debug,
				Timeout:          common.Timeout,
				GracefulShutdown: common.Graceful,
			},
		),
		clab.WithDebug(common.Debug),
	}

	if common.Topo != "" {
		opts = append(opts,
			clab.WithTopoPath(common.Topo, common.VarsFile),
			clab.WithNodeFilter(common.NodeFilter),
		)
	}

	c, err := clab.NewContainerLab(opts...)
	if err != nil {
		return fmt.Errorf("could not parse the topology file: %v", err)
	}

	err = c.CheckConnectivity(ctx)
	if err != nil {
		return err
	}

	var containers []clabRuntime.GenericContainer
	var glabels []*types.GenericFilter

	labName := ""
	if common.Name != "" {
		labName = common.Name
	} else if c.Config.Name != "" {
		labName = c.Config.Name
	} else {
		return fmt.Errorf("could not find topology")
	}

	glabels = append(glabels, &types.GenericFilter{
		FilterType: "label", Match: labName,
		Field: labels.Containerlab, Operator: "=",
	})

	if interfacesNodeName != "" {
		glabels = append(glabels, &types.GenericFilter{
			FilterType: "label", Match: strings.TrimPrefix(interfacesNodeName, "clab-"+labName+"-"),
			Field: labels.NodeName, Operator: "=",
		})
	}

	containers, err = c.ListContainers(ctx, glabels)
	if err != nil {
		return fmt.Errorf("failed to list containers: %s", err)
	}

	if len(containers) == 0 {
		log.Println("no containers found")
		return nil
	}

	err = printContainerInterfaces(ctx, containers, interfacesFormat)
	return err
}

func getContainerInterfaces(ctx context.Context, rt clabRuntime.ContainerRuntime, container clabRuntime.GenericContainer) (*types.ContainerInterfaces, error) {
	containerInterfaces := types.ContainerInterfaces{}

	if len(container.Names) > 0 {
		containerInterfaces.ContainerName = container.Names[0]
	}

	containerInterfaces.Interfaces = make([]*types.ContainerInterfaceDetails, 0)

	// retrieve the containers NSPath
	nodeNsPath, err := rt.GetNSPath(ctx, containerInterfaces.ContainerName)
	if err != nil {
		return nil, err
	}

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Get handle for NS
	nsHandle, err := netlinkNs.GetFromPath(nodeNsPath)
	if err != nil {
		return nil, err
	}

	netlinkHandle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, err
	}

	interfaces, err := netlinkHandle.LinkList()

	for _, iface := range interfaces {
		ifaceDetails := types.ContainerInterfaceDetails{}
		ifaceDetails.InterfaceName = iface.Attrs().Name
		ifaceDetails.InterfaceAlias = iface.Attrs().Alias
		ifaceDetails.InterfaceType = iface.Type()
		ifaceDetails.InterfaceState = iface.Attrs().OperState.String()

		containerInterfaces.Interfaces = append(containerInterfaces.Interfaces, &ifaceDetails)
	}

	return &containerInterfaces, nil
}

func interfacestoTableData(contInterfaces []*types.ContainerInterfaces) []tableWriter.Row {
	tabData := make([]tableWriter.Row, 0, 0)
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
				iface.InterfaceType,
				iface.InterfaceState,
			)

			tabData = append(tabData, tabRow)
		}
	}
	return tabData
}

func printContainerInterfaces(ctx context.Context, containers []clabRuntime.GenericContainer, format string) error {
	contInterfaces := make([]*types.ContainerInterfaces, 0, len(containers))

	// Get the runtime initializer.
	_, rinit, err := clab.RuntimeInitializer(common.Runtime)
	if err != nil {
		return err
	}

	// init the runtime
	rt := rinit()

	// init runtime with timeout
	err = rt.Init(
		clabRuntime.WithConfig(
			&clabRuntime.RuntimeConfig{
				Timeout: common.Timeout,
			},
		),
	)
	if err != nil {
		return err
	}

	// Gather interface information for each container
	for _, cont := range containers {
		cIfs, err := getContainerInterfaces(ctx, rt, cont)
		if err != nil {
			return fmt.Errorf("error getting container interfaces for %v: %w", cIfs.ContainerName, err)
		}

		sort.Slice(cIfs.Interfaces, func(i, j int) bool {
			return cIfs.Interfaces[i].InterfaceName < cIfs.Interfaces[j].InterfaceName
		})

		contInterfaces = append(contInterfaces, cIfs)
	}

	sort.Slice(contInterfaces, func(i, j int) bool {
		return contInterfaces[i].ContainerName < contInterfaces[j].ContainerName
	})

	switch format {
	case "json":
		b, err := json.MarshalIndent(contInterfaces, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal container details: %v", err)
		}
		fmt.Println(string(b))
		return nil

	case "table":
		tabData := interfacestoTableData(contInterfaces)
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
			"Interface Name",
			"Interface Alias",
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
				}},
		})

		table.AppendRows(tabData)

		table.Render()

		return nil
	}
	return nil
}

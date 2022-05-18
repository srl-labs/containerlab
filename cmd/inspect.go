// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/mysocketio"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var format string
var details bool
var all bool

// inspectCmd represents the inspect command
var inspectCmd = &cobra.Command{
	Use:     "inspect",
	Short:   "inspect lab details",
	Long:    "show details about a particular lab or all running labs\nreference: https://containerlab.dev/cmd/inspect/",
	Aliases: []string{"ins", "i"},
	PreRunE: sudoCheck,
	RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" && topo == "" && !all {
			fmt.Println("provide either a lab name (--name) or a topology file path (--topo) or the flag --all")
			return nil
		}
		opts := []clab.ClabOption{
			clab.WithTimeout(timeout),
			clab.WithRuntime(rt,
				&runtime.RuntimeConfig{
					Debug:            debug,
					Timeout:          timeout,
					GracefulShutdown: graceful,
				},
			),
		}
		if topo != "" {
			opts = append(opts, clab.WithTopoFile(topo, varsFile))
		}
		c, err := clab.NewContainerLab(opts...)
		if err != nil {
			return fmt.Errorf("could not parse the topology file: %v", err)
		}

		if name == "" {
			name = c.Config.Name
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var glabels []*types.GenericFilter
		if all {
			glabels = []*types.GenericFilter{{FilterType: "label", Field: "containerlab", Operator: "exists"}}
		} else {
			if name != "" {
				glabels = []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
			} else if topo != "" {
				glabels = []*types.GenericFilter{{FilterType: "label", Match: c.Config.Name, Field: "containerlab", Operator: "="}}
			}
		}

		containers, err := c.ListContainers(ctx, glabels)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}

		if details {
			b, err := json.MarshalIndent(containers, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal containers struct: %v", err)
			}
			fmt.Println(string(b))
			return nil
		}

		err = printContainerInspect(c, containers, format)
		return err
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().BoolVarP(&details, "details", "", false, "print all details of lab containers")
	inspectCmd.Flags().StringVarP(&format, "format", "f", "table", "output format. One of [table, json]")
	inspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
}

func toTableData(det []types.ContainerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for i := range det {
		d := &det[i]

		if all {
			tabData = append(tabData, []string{fmt.Sprintf("%d", i+1), d.LabPath, d.LabName, d.Name, d.ContainerID, d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address})
			continue
		}
		tabData = append(tabData, []string{fmt.Sprintf("%d", i+1), d.Name, d.ContainerID, d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address})
	}
	return tabData
}

func printContainerInspect(c *clab.CLab, containers []types.GenericContainer, format string) error {

	if len(containers) == 0 && format == "table" {
		fmt.Println("no containers found")
		return nil
	}

	contDetails := make([]types.ContainerDetails, 0, len(containers))
	// do not print published ports unless mysocketio kind is found
	printMysocket := false

	// Gather details of each container
	for i := range containers {
		cont := &containers[i]
		// get topo file path relative of the cwd
		cwd, _ := os.Getwd()
		path, _ := filepath.Rel(cwd, cont.Labels["clab-topo-file"])

		cdet := &types.ContainerDetails{
			LabName:     cont.Labels["containerlab"],
			LabPath:     path,
			Image:       cont.Image,
			State:       cont.State,
			IPv4Address: cont.GetContainerIPv4(),
			IPv6Address: cont.GetContainerIPv6(),
		}
		cdet.ContainerID = cont.ShortID

		if len(cont.Names) > 0 {
			cdet.Name = strings.TrimLeft(cont.Names[0], "/")
		}
		if kind, ok := cont.Labels["clab-node-kind"]; ok {
			cdet.Kind = kind
			if kind == "mysocketio" {
				printMysocket = true
			}
		}
		if group, ok := cont.Labels["clab-node-group"]; ok {
			cdet.Group = group
		}
		contDetails = append(contDetails, *cdet)
	}

	sort.Slice(contDetails, func(i, j int) bool {
		if contDetails[i].LabName == contDetails[j].LabName {
			return contDetails[i].Name < contDetails[j].Name
		}
		return contDetails[i].LabName < contDetails[j].LabName
	})

	resultJson := &types.LabData{Containers: contDetails, MySocketIo: []*types.MySocketIoEntry{}}
	var socketdata []*types.MySocketIoEntry
	var tokenFile string
	var err error

	// fetch mysocketio data if mysocketio node is detected to present in a list of nodes and nodes are not empty
	// nodes are not populated when `inspect --all` is used, since we don't read topology files
	if printMysocket && len(c.Nodes) != 0 {
		// get mysocketio token file path by fetching it from the mysocketio node' binds section
		tokenFile, err = mySocketIoTokenFileFromBindMounts(c.Nodes, c.TopoFile.GetDir())
		if err != nil {
			return err
		}
		// retrieve the MySocketIO Data
		socketdata, err = getMySocketIoData(tokenFile)
		if err != nil {
			return fmt.Errorf("error when processing mysocketio data: %v", err)
		}
		resultJson.MySocketIo = socketdata
	}

	switch format {
	case "json":
		b, err := json.MarshalIndent(resultJson, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal container details: %v", err)
		}
		fmt.Println(string(b))
		return nil

	case "table":
		tabData := toTableData(contDetails)
		table := tablewriter.NewWriter(os.Stdout)
		header := []string{
			"Lab Name",
			"Name",
			"Container ID",
			"Image",
			"Kind",
			"State",
			"IPv4 Address",
			"IPv6 Address",
		}
		if all {
			table.SetHeader(append([]string{"#", "Topo Path"}, header...))
		} else {
			table.SetHeader(append([]string{"#"}, header[1:]...))
		}
		table.SetAutoFormatHeaders(false)
		table.SetAutoWrapText(false)
		// merge cells with lab name and topo file path
		table.SetAutoMergeCellsByColumnIndex([]int{1, 2})
		table.AppendBulk(tabData)
		table.Render()

		// do not print mysocket data if printMysocket is false or we don't have nodes populated
		// nodes are not populated when `inspect --all` is used, since we don't read topology files
		if !printMysocket || len(c.Nodes) == 0 {
			return nil
		}

		// prepare data for table
		var tabDataMySocketIo [][]string
		for _, entry := range socketdata {
			var portstrarr []string
			for _, port := range entry.Ports {
				portstrarr = append(portstrarr, strconv.Itoa(port))
			}
			tabDataMySocketIo = append(tabDataMySocketIo, []string{*entry.SocketId, *entry.DnsName, strings.Join(portstrarr, ", "), *entry.Type, strconv.FormatBool(entry.CloudAuth), *entry.Name})
		}
		tableMySocketIo := tablewriter.NewWriter(os.Stdout)
		headerMySocketIo := []string{
			"Socket ID",
			"DNS Name",
			"Ports",
			"Type",
			"Cloud Auth",
			"Name",
		}
		// configure table output
		tableMySocketIo.SetHeader(headerMySocketIo)
		tableMySocketIo.SetAutoFormatHeaders(false)
		tableMySocketIo.SetAutoWrapText(false)
		tableMySocketIo.AppendBulk(tabDataMySocketIo)
		fmt.Println("Published ports:")
		tableMySocketIo.Render()

		return nil
	}
	return nil
}

// getMySocketioData uses the mysocketio.http client to retrieve the socket data
func getMySocketIoData(tokenfile string) ([]*types.MySocketIoEntry, error) {
	result := []*types.MySocketIoEntry{}

	client, err := mysocketio.NewClient(tokenfile)
	if err != nil {
		return nil, err
	}

	sockets := []mysocketio.Socket{}
	err = client.Request("GET", "connect", &sockets, nil)
	if err != nil {
		return nil, err
	}

	for i := range sockets {
		newentry := &types.MySocketIoEntry{
			SocketId:  &sockets[i].SocketID,
			DnsName:   &sockets[i].Dnsname,
			Ports:     sockets[i].SocketTcpPorts,
			Type:      &sockets[i].SocketType,
			CloudAuth: sockets[i].CloudAuthEnabled,
			Name:      &sockets[i].Name,
		}
		result = append(result, newentry)
	}
	return result, nil
}

// mySocketIoTokenFileFromBindMounts finds a node of kind mysocketio.
// if that is found, the bindmounts are searched for ".mysocketio_token" and the path is being converted into an
// absolute path and returned.
func mySocketIoTokenFileFromBindMounts(_nodes map[string]nodes.Node, configPath string) (string, error) {
	// if not mysocketio kind then continue
	var mysocketNode nodes.Node
	var ok bool

	if mysocketNode, ok = _nodes["mysocketio"]; !ok {
		return "", fmt.Errorf("no mysocketio node found")
	}
	// if "mysocketio" kind then iterate through bind mounts
	for _, bind := range mysocketNode.Config().Binds {
		// look for ".mysocketio_token"
		if strings.Contains(bind, ".mysocketio_token") {
			// split the bindmount and resolve the path to an absolute path
			deduced_absfilepath := utils.ResolvePath(strings.Split(bind, ":")[0], configPath)
			// check file existence before returning
			if !utils.FileExists(deduced_absfilepath) {
				return "", fmt.Errorf(".mysocketio_token resolved to %s, but that file doesn't exist", deduced_absfilepath)
			}
			return deduced_absfilepath, nil
		}
	}

	return "", fmt.Errorf("unable to find \".mysocketio_token\"")
}

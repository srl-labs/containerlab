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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/internal/slices"
	"github.com/srl-labs/containerlab/mysocketio"
	mysocketionode "github.com/srl-labs/containerlab/nodes/mysocketio"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

var (
	format  string
	details bool
	all     bool
)

// inspectCmd represents the inspect command.
var inspectCmd = &cobra.Command{
	Use:     "inspect",
	Short:   "inspect lab details",
	Long:    "show details about a particular lab or all running labs\nreference: https://containerlab.dev/cmd/inspect/",
	Aliases: []string{"ins", "i"},
	PreRunE: sudoCheck,
	RunE:    inspectFn,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().BoolVarP(&details, "details", "", false, "print all details of lab containers")
	inspectCmd.Flags().StringVarP(&format, "format", "f", "table", "output format. One of [table, json]")
	inspectCmd.Flags().BoolVarP(&all, "all", "a", false, "show all deployed containerlab labs")
}

func inspectFn(_ *cobra.Command, _ []string) error {
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

	var containers []types.GenericContainer

	// if the topo file is availabel, use it
	if topo != "" {
		containers, err = c.ListContainersClabNodes(ctx)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}
	} else {
		var glabels []*types.GenericFilter
		// or when just the name is given
		if name != "" {
			// if name is set, filter for name
			glabels = []*types.GenericFilter{{FilterType: "label", Match: name, Field: "containerlab", Operator: "="}}
		} else {
			// this is the --all case
			glabels = []*types.GenericFilter{{FilterType: "label", Field: "containerlab", Operator: "exists"}}
		}
		containers, err = c.ListContainers(ctx, glabels)
		if err != nil {
			return fmt.Errorf("failed to list containers: %s", err)
		}
	}

	if len(containers) == 0 {
		log.Println("no containers found")
		return nil
	}
	if details {
		b, err := json.MarshalIndent(containers, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal containers struct: %v", err)
		}
		fmt.Println(string(b))
		return nil
	}

	err = printContainerInspect(containers, format)
	return err
}

func toTableData(det []types.ContainerDetails) [][]string {
	tabData := make([][]string, 0, len(det))
	for i := range det {
		d := &det[i]

		if all {
			tabData = append(tabData, []string{
				fmt.Sprintf("%d", i+1), d.LabPath,
				d.LabName, d.Name, d.ContainerID, d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address,
			})
			continue
		}
		tabData = append(tabData, []string{
			fmt.Sprintf("%d", i+1), d.Name, d.ContainerID,
			d.Image, d.Kind, d.State, d.IPv4Address, d.IPv6Address,
		})
	}
	return tabData
}

func printContainerInspect(containers []types.GenericContainer, format string) error {
	contDetails := make([]types.ContainerDetails, 0, len(containers))
	// do not print published ports unless mysocketio kind is found
	printMysocket := false

	// Gather details of each container
	for _, cont := range containers {

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
			cdet.Name = cont.Names[0]
		}
		if kind, ok := cont.Labels["clab-node-kind"]; ok {
			cdet.Kind = kind
			if slices.Contains(mysocketionode.Kindnames, kind) {
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

	resultData := &types.LabData{Containers: contDetails, MySocketIo: []*types.MySocketIoEntry{}}
	var socketdata []*types.MySocketIoEntry
	var tokenFiles []*TokenFileResults
	var err error

	// fetch mysocketio data if mysocketio node is detected to present in a list of nodes and nodes are not empty
	// nodes are not populated when `inspect --all` is used, since we don't read topology files
	if printMysocket {
		// get mysocketio token file path by fetching it from the mysocketio node' binds section
		tokenFiles = mySocketIoTokenFileFromBindMounts(containers)
		for _, tokendata := range tokenFiles {
			// retrieve the MySocketIO Data
			socketdata, err = getMySocketIoData(tokendata.File)
			if err != nil {
				return fmt.Errorf("error when processing mysocketio data: %v", err)
			}
			for _, entry := range socketdata {
				entry.LabName = &tokendata.Labname
			}
			resultData.MySocketIo = append(resultData.MySocketIo, socketdata...)
		}
	}

	switch format {
	case "json":
		b, err := json.MarshalIndent(resultData, "", "  ")
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
		if !printMysocket {
			return nil
		}

		// prepare data for table
		var tabDataMySocketIo [][]string
		for _, entry := range resultData.MySocketIo {
			var portstrarr []string
			for _, port := range entry.Ports {
				portstrarr = append(portstrarr, strconv.Itoa(port))
			}
			tabDataMySocketIo = append(tabDataMySocketIo, []string{
				*entry.LabName, *entry.Name, *entry.SocketId, *entry.DnsName,
				strings.Join(portstrarr, ", "), *entry.Type, strconv.FormatBool(entry.CloudAuth),
			})
		}
		tableMySocketIo := tablewriter.NewWriter(os.Stdout)
		headerMySocketIo := []string{
			"Lab Name",
			"Name",
			"Socket ID",
			"DNS Name",
			"Ports",
			"Type",
			"Cloud Auth",
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

// getMySocketioData uses the mysocketio.http client to retrieve the socket data.
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

type TokenFileResults struct {
	File    string
	Labname string
}

// mySocketIoTokenFileFromBindMounts runs through the provided slice of GenericContainers to deduce the mysocketio containers
// if those are found the bindmounts are evaluated to find the hostpath to the referenced ".mysocketio_token" files. Since multiple
// labs might be started the result is a slice of "".mysocketio_token" files.
func mySocketIoTokenFileFromBindMounts(containers []types.GenericContainer) []*TokenFileResults {
	result := []*TokenFileResults{}
	for _, node := range containers {
		// if not mysocketio kind then continue
		if !slices.Contains(mysocketionode.Kindnames, node.Labels["clab-node-kind"]) {
			continue
		}

		filepath := ""
		// if "mysocketio" kind then iterate through bind mounts
		for _, bind := range node.Mounts {
			// watch out for ".mysocketio_token"
			if strings.Contains(bind.Destination, ".mysocketio_token") {
				filepath = bind.Source
			}
		}
		// check we found the ".mysocketio_token"
		if filepath == "" {
			log.Warningf("Skipping Node %s, although it seems to be mysocketio node. Unable to determine referenced \".mysocketio_token\" file. ", node.ID)
			continue
		}
		result = append(result, &TokenFileResults{
			Labname: node.Labels["containerlab"],
			File:    filepath,
		})
	}
	return result
}

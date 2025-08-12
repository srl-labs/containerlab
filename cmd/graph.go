// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"html/template"
	"sort"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	containerlabcore "github.com/srl-labs/containerlab/core"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	containerlabtypes "github.com/srl-labs/containerlab/types"
)

var (
	srv              string
	tmpl             string
	offline          bool
	dot              bool
	mermaid          bool
	mermaidDirection string
	drawio           bool
	drawioVersion    string
	drawioArgs       []string
	staticDir        string
)

// graphCmd represents the graph command.
var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "generate a topology graph",
	Long:  "generate topology graph based on the topology definition file and running containers\nreference: https://containerlab.dev/cmd/graph/",
	RunE:  graphFn,
}

func graphFn(_ *cobra.Command, _ []string) error {
	var err error

	opts := []containerlabcore.ClabOption{
		containerlabcore.WithTimeout(timeout),
		containerlabcore.WithTopoPath(topoFile, varsFile),
		containerlabcore.WithNodeFilter(nodeFilter),
		containerlabcore.WithRuntime(
			runtime,
			&containerlabruntime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		containerlabcore.WithDebug(debug),
	}
	c, err := containerlabcore.NewContainerLab(opts...)
	if err != nil {
		return err
	}

	err = c.ResolveLinks()
	if err != nil {
		return err
	}

	if dot {
		return c.GenerateDotGraph()
	}

	if mermaid {
		return c.GenerateMermaidGraph(mermaidDirection)
	}

	if drawio {
		return c.GenerateDrawioDiagram(drawioVersion, drawioArgs)
	}

	gtopo := containerlabcore.GraphTopo{
		Nodes: make([]containerlabtypes.ContainerDetails, 0, len(c.Nodes)),
		Links: make([]containerlabcore.Link, 0, len(c.Links)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var containers []containerlabruntime.GenericContainer
	// if offline mode is not enforced, list containers matching lab name
	if !offline {
		containers, err = c.ListContainers(ctx,
			containerlabcore.WithListLabName(c.Config.Name))
		if err != nil {
			return err
		}

		log.Debugf("found %d containers", len(containers))
	}

	switch {
	case len(containers) == 0:
		c.BuildGraphFromTopo(&gtopo)
	case len(containers) > 0:
		c.BuildGraphFromDeployedLab(&gtopo, containers)
	}

	sort.Slice(gtopo.Nodes, func(i, j int) bool {
		return gtopo.Nodes[i].Name < gtopo.Nodes[j].Name
	})
	for _, l := range c.Links {
		eps := l.GetEndpoints()

		ifaceDisplayNameA := eps[0].GetIfaceDisplayName()
		ifaceDisplayNameB := eps[1].GetIfaceDisplayName()

		gtopo.Links = append(gtopo.Links, containerlabcore.Link{
			Source:         eps[0].GetNode().GetShortName(),
			SourceEndpoint: ifaceDisplayNameA,
			Target:         eps[1].GetNode().GetShortName(),
			TargetEndpoint: ifaceDisplayNameB,
		})
	}

	b, err := json.Marshal(gtopo)
	if err != nil {
		return err
	}

	log.Debugf("generating graph using data: %s", string(b))
	topoD := containerlabcore.TopoData{
		Name: c.Config.Name,
		Data: template.JS(string(b)), // skipcq: GSC-G203
	}

	return c.ServeTopoGraph(tmpl, staticDir, srv, topoD)
}

func init() {
	RootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&srv, "srv", "s", "0.0.0.0:50080",
		"HTTP server address serving the topology view")
	graphCmd.Flags().BoolVarP(&offline, "offline", "o", false,
		"use only information from topo file when building graph")
	graphCmd.Flags().BoolVarP(&dot, "dot", "", false, "generate dot file")
	graphCmd.Flags().BoolVarP(&mermaid, "mermaid", "", false, "print mermaid flowchart to stdout")
	graphCmd.Flags().StringVarP(&mermaidDirection, "mermaid-direction", "", "TD", "specify direction of mermaid dirgram")
	graphCmd.Flags().StringSliceVar(&drawioArgs, "drawio-args", []string{},
		"Additional flags to pass to the drawio diagram generation tool (can be specified multiple times)")
	graphCmd.Flags().BoolVarP(&drawio, "drawio", "", false, "generate drawio diagram file")
	graphCmd.Flags().StringVarP(&drawioVersion, "drawio-version", "", "latest",
		"version of the clab-io-draw container to use for generating drawio diagram file")
	graphCmd.Flags().StringVarP(&tmpl, "template", "", "",
		"Go html template used to generate the graph")
	graphCmd.Flags().StringVarP(&staticDir, "static-dir", "", "",
		"Serve static files from the specified directory")
	graphCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	graphCmd.MarkFlagsMutuallyExclusive("dot", "mermaid", "drawio")
}

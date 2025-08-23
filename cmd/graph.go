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
	clabcore "github.com/srl-labs/containerlab/core"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func graphCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "graph",
		Short: "generate a topology graph",
		Long: "generate topology graph based on the topology definition file and " +
			"running containers\nreference: https://containerlab.dev/cmd/graph/",
		RunE: func(_ *cobra.Command, _ []string) error {
			return graphFn(o)
		},
	}

	c.Flags().StringVarP(
		&o.Graph.Server,
		"srv",
		"s",
		o.Graph.Server,
		"HTTP server address serving the topology view",
	)
	c.Flags().BoolVarP(
		&o.Graph.Offline,
		"offline",
		"o",
		o.Graph.Offline,
		"use only information from topo file when building graph",
	)
	c.Flags().BoolVarP(
		&o.Graph.GenerateDotFile,
		"dot",
		"",
		o.Graph.GenerateDotFile,
		"generate dot file",
	)
	c.Flags().BoolVarP(
		&o.Graph.GenerateMermaid,
		"mermaid",
		"",
		o.Graph.GenerateMermaid,
		"print mermaid flowchart to stdout",
	)
	c.Flags().StringVarP(
		&o.Graph.MermaidDirection,
		"mermaid-direction", "",
		o.Graph.MermaidDirection,
		"specify direction of mermaid dirgram",
	)
	c.Flags().StringSliceVar(
		&o.Graph.DrawIOArgs,
		"drawio-args",
		o.Graph.DrawIOArgs,
		"Additional flags to pass to the drawio diagram generation tool "+
			"(can be specified multiple times)",
	)
	c.Flags().BoolVarP(
		&o.Graph.GenerateDrawIO,
		"drawio",
		"",
		o.Graph.GenerateDrawIO,
		"generate drawio diagram file",
	)
	c.Flags().StringVarP(
		&o.Graph.DrawIOVersion,
		"drawio-version",
		"",
		o.Graph.DrawIOVersion,
		"version of the clab-io-draw container to use for generating drawio diagram file",
	)
	c.Flags().StringVarP(
		&o.Graph.Template,
		"template",
		"",
		o.Graph.Template,
		"Go html template used to generate the graph",
	)
	c.Flags().StringVarP(
		&o.Graph.StaticDirectory,
		"static-dir",
		"",
		o.Graph.StaticDirectory,
		"Serve static files from the specified directory",
	)
	c.Flags().StringSliceVarP(
		&o.Filter.NodeFilter,
		"node-filter",
		"",
		o.Filter.NodeFilter,
		"comma separated list of nodes to include",
	)
	c.MarkFlagsMutuallyExclusive("dot", "mermaid", "drawio")

	return c, nil
}

func graphFn(o *Options) error {
	c, err := clabcore.NewContainerLab(o.ToClabOptions()...)
	if err != nil {
		return err
	}

	err = c.ResolveLinks()
	if err != nil {
		return err
	}

	if o.Graph.GenerateDotFile {
		return c.GenerateDotGraph()
	}

	if o.Graph.GenerateMermaid {
		return c.GenerateMermaidGraph(o.Graph.MermaidDirection)
	}

	if o.Graph.GenerateDrawIO {
		return c.GenerateDrawioDiagram(o.Graph.DrawIOVersion, o.Graph.DrawIOArgs)
	}

	gtopo := clabcore.GraphTopo{
		Nodes: make([]clabtypes.ContainerDetails, 0, len(c.Nodes)),
		Links: make([]clabcore.Link, 0, len(c.Links)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var containers []clabruntime.GenericContainer
	// if offline mode is not enforced, list containers matching lab name
	if !o.Graph.Offline {
		containers, err = c.ListContainers(ctx,
			clabcore.WithListLabName(c.Config.Name))
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

		gtopo.Links = append(gtopo.Links, clabcore.Link{
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
	topoD := clabcore.TopoData{
		Name: c.Config.Name,
		Data: template.JS(string(b)), // skipcq: GSC-G203
	}

	return c.ServeTopoGraph(o.Graph.Template, o.Graph.StaticDirectory, o.Graph.Server, topoD)
}

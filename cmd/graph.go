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

func graphCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   "graph",
		Short: "generate a topology graph",
		Long:  "generate topology graph based on the topology definition file and running containers\nreference: https://containerlab.dev/cmd/graph/",
		RunE: func(_ *cobra.Command, _ []string) error {
			return graphFn(o)
		},
	}

	c.Flags().StringVarP(&srv, "srv", "s", "0.0.0.0:50080",
		"HTTP server address serving the topology view")
	c.Flags().BoolVarP(&offline, "offline", "o", false,
		"use only information from topo file when building graph")
	c.Flags().BoolVarP(&dot, "dot", "", false, "generate dot file")
	c.Flags().BoolVarP(&mermaid, "mermaid", "", false, "print mermaid flowchart to stdout")
	c.Flags().StringVarP(&mermaidDirection, "mermaid-direction", "", "TD", "specify direction of mermaid dirgram")
	c.Flags().StringSliceVar(&drawioArgs, "drawio-args", []string{},
		"Additional flags to pass to the drawio diagram generation tool (can be specified multiple times)")
	c.Flags().BoolVarP(&drawio, "drawio", "", false, "generate drawio diagram file")
	c.Flags().StringVarP(&drawioVersion, "drawio-version", "", "latest",
		"version of the clab-io-draw container to use for generating drawio diagram file")
	c.Flags().StringVarP(&tmpl, "template", "", "",
		"Go html template used to generate the graph")
	c.Flags().StringVarP(&staticDir, "static-dir", "", "",
		"Serve static files from the specified directory")
	c.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	c.MarkFlagsMutuallyExclusive("dot", "mermaid", "drawio")

	return c, nil
}

func graphFn(o *Options) error {
	var err error

	opts := []clabcore.ClabOption{
		clabcore.WithTimeout(o.Global.Timeout),
		clabcore.WithTopoPath(o.Global.TopologyFile, o.Global.VarsFile),
		clabcore.WithNodeFilter(nodeFilter),
		clabcore.WithRuntime(
			o.Global.Runtime,
			&clabruntime.RuntimeConfig{
				Debug:            o.Global.DebugCount > 0,
				Timeout:          o.Global.Timeout,
				GracefulShutdown: gracefulShutdown,
			},
		),
		clabcore.WithDebug(o.Global.DebugCount > 0),
	}
	c, err := clabcore.NewContainerLab(opts...)
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

	gtopo := clabcore.GraphTopo{
		Nodes: make([]clabtypes.ContainerDetails, 0, len(c.Nodes)),
		Links: make([]clabcore.Link, 0, len(c.Links)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var containers []clabruntime.GenericContainer
	// if offline mode is not enforced, list containers matching lab name
	if !offline {
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

	return c.ServeTopoGraph(tmpl, staticDir, srv, topoD)
}

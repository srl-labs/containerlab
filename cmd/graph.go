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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/srl-labs/containerlab/clab"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	defaultGraphTemplatePath = "/etc/containerlab/templates/graph/nextui/nextui.html"
	defaultStaticPath        = "/etc/containerlab/templates/graph/nextui/static"
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

	opts := []clab.ClabOption{
		clab.WithTimeout(timeout),
		clab.WithTopoPath(topo, varsFile),
		clab.WithNodeFilter(nodeFilter),
		clab.WithRuntime(rt,
			&runtime.RuntimeConfig{
				Debug:            debug,
				Timeout:          timeout,
				GracefulShutdown: graceful,
			},
		),
		clab.WithDebug(debug),
	}
	c, err := clab.NewContainerLab(opts...)
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
		return c.GenerateDrawioDiagram(drawioVersion)
	}

	gtopo := clab.GraphTopo{
		Nodes: make([]types.ContainerDetails, 0, len(c.Nodes)),
		Links: make([]clab.Link, 0, len(c.Links)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var containers []runtime.GenericContainer
	// if offline mode is not enforced, list containers matching lab name
	if !offline {
		labels := []*types.GenericFilter{{
			FilterType: "label", Match: c.Config.Name,
			Field: labels.Containerlab, Operator: "=",
		}}
		containers, err = c.ListContainers(ctx, labels)
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

		gtopo.Links = append(gtopo.Links, clab.Link{
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
	topoD := clab.TopoData{
		Name: c.Config.Name,
		Data: template.JS(string(b)), // skipcq: GSC-G203
	}

	return c.ServeTopoGraph(tmpl, staticDir, srv, topoD)
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&srv, "srv", "s", "0.0.0.0:50080",
		"HTTP server address serving the topology view")
	graphCmd.Flags().BoolVarP(&offline, "offline", "o", false,
		"use only information from topo file when building graph")
	graphCmd.Flags().BoolVarP(&dot, "dot", "", false, "generate dot file")
	graphCmd.Flags().BoolVarP(&mermaid, "mermaid", "", false, "print mermaid flowchart to stdout")
	graphCmd.Flags().StringVarP(&mermaidDirection, "mermaid-direction", "", "TD", "specify direction of mermaid dirgram")
	graphCmd.Flags().BoolVarP(&drawio, "drawio", "", false, "generate drawio diagram file")
	graphCmd.Flags().StringVarP(&drawioVersion, "drawio-version", "", "latest",
		"version of the clab-io-draw container to use for generating drawio diagram file")
	graphCmd.Flags().StringVarP(&tmpl, "template", "", defaultGraphTemplatePath,
		"Go html template used to generate the graph")
	graphCmd.Flags().StringVarP(&staticDir, "static-dir", "", defaultStaticPath,
		"Serve static files from the specified directory")
	graphCmd.Flags().StringSliceVarP(&nodeFilter, "node-filter", "", []string{},
		"comma separated list of nodes to include")
	graphCmd.MarkFlagsMutuallyExclusive("dot", "mermaid", "drawio")
}

package clab

import (
	"bytes"
	"strings"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	log "github.com/sirupsen/logrus"
)

// GenerateGraph generates a graph for the lab topology
func (c *cLab) GenerateGraph(topo string) error {
	h := graphviz.New()
	graph, err := h.Graph()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		h.Close()
	}()

	graph.Attr(1, "style", "filled")  // kind 1 == node defaults
	graph.Attr(1, "fillcolor", "red") // kind 1 == node defaults

	var graphNodeMap = make(map[string]*cgraph.Node)

	// Create Nodes
	for nodeName, node := range c.Nodes {
		n, err := graph.CreateNode(nodeName)
		if err != nil {
			log.Fatal(err)
		}
		n.SetLabel(nodeName)
		n.SetXLabel(node.Kind)
		n.SetGroup(node.Group)

		if strings.Contains(node.Kind, "srl") {
			n.SetFillColor("green")
		}
		if strings.Contains(node.Group, "bb") {
			n.SetFillColor("blue")
			n.SetFontColor("white")
		}

		graphNodeMap[nodeName] = n
	}

	// Create Edges between the Nodes
	graph.Attr(2, "fillcolor", "green") // kind 2 == edge defaults
	graph.Attr(2, "arrowhead", "none")  // kind 2 == edge defaults
	graph.Attr(2, "arrowtail", "none")  // kind 2 == edge defaults
	for _, link := range c.Links {
		aEnd, ok := graphNodeMap[link.A.Node.ShortName]
		if !ok {
			log.Fatal("Node ", link.A.EndpointName, " does not exist!")
		}
		bEnd, ok := graphNodeMap[link.B.Node.ShortName]
		if !ok {
			log.Fatal("Node ", link.B.EndpointName, " does not exist!")
		}
		e, err := graph.CreateEdge("", aEnd, bEnd)
		if err != nil {
			log.Fatal(err)
		}
		_ = e
		// Print node and interface name in edge label
		//e.SetLabel(link.A.Node.ShortName + " " + link.A.EndpointName + "\n" + link.B.Node.ShortName + " " + link.B.EndpointName)
	}

	// create graph directory
	CreateDirectory(c.Dir.Lab, 0755)
	CreateDirectory(c.Dir.LabGraph, 0755)

	// write .dot file
	var buf bytes.Buffer
	if err := h.Render(graph, "dot", &buf); err != nil {
		log.Fatal(err)
	}
	//fmt.Println(buf.String())
	dotFile := c.Dir.LabGraph + "/" + c.FileInfo.name + ".dot"
	createFile(dotFile, buf.String())

	pngFile := c.Dir.LabGraph + "/" + c.FileInfo.name + ".png"
	if err := h.RenderFilename(graph, graphviz.PNG, pngFile); err != nil {
		log.Fatal(err)
	}
	log.Info("Done")
	return nil
}

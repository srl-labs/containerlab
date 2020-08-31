package main

import (
	"strings"

	"github.com/awalterschulze/gographviz"
)

var g *gographviz.Graph

func (c *cLab) generateGraph(topo string) error {
	g = gographviz.NewGraph()
	if err := g.SetName(FileInfo.shortname); err != nil {
		return err
	}
	if err := g.SetDir(false); err != nil {
		return err
	}

	var attr map[string]string
	attr = make(map[string]string)
	attr["color"] = "red"
	attr["style"] = "filled"
	attr["fillcolor"] = "red"

	for nodeName, node := range c.Nodes {
		attr["label"] = nodeName
		attr["xlabel"] = node.OS
		attr["group"] = node.Group

		if strings.Contains(node.OS, "srl") {
			attr["fillcolor"] = "green"
		}
		if strings.Contains(node.Group, "bb") {
			attr["fillcolor"] = "blue"
		}
		if err := g.AddNode(FileInfo.shortname, node.Name, attr); err != nil {
			return err
		}

	}

	attr = make(map[string]string)
	attr["color"] = "green"

	for _, link := range c.Links {
		if strings.Contains(link.b.Node.Name, "client") {
			attr["color"] = "blue"
		}
		if err := g.AddEdge(link.a.Node.Name, link.b.Node.Name, false, attr); err != nil {
			return err
		}

	}

	// create graph directory
	path := c.Conf.ConfigPath + "/" + "graph"
	createDirectory(path, 0755)

	// create graph filename
	file := path + "/" + FileInfo.name + ".dot"

	createFile(file, g.String())

	//s := g.String()
	//fmt.Println(s)

	return nil
}
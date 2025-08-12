package mermaid

import (
	"fmt"
	"io"

	"golang.org/x/exp/slices"
)

// A very minimalistic Mermaid flowchart generator
// that covers the usecase of `containerlab graph`
// command.

type FlowChart struct {
	title     string
	direction string
	edges     []Edge
}

type Edge struct {
	nodeA string
	nodeB string
}

func NewFlowChart() *FlowChart {
	return &FlowChart{
		edges: []Edge{},
	}
}

func (fc *FlowChart) SetTitle(title string) {
	fc.title = title
}

func (fc *FlowChart) SetDirection(direction string) error {
	validDirections := []string{"TB", "TD", "BT", "RL", "LR"}
	if !slices.Contains(validDirections, direction) {
		return fmt.Errorf("invalid direction %s (should be one of %v)", direction, validDirections)
	}
	fc.direction = direction
	return nil
}

func (fc *FlowChart) AddEdge(nodeA, nodeB string) {
	fc.edges = append(fc.edges, Edge{nodeA: nodeA, nodeB: nodeB})
}

func (fc *FlowChart) Generate(w io.Writer) {
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "title: %s\n", fc.title)
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "graph %s\n", fc.direction)
	for _, edge := range fc.edges {
		fmt.Fprintf(w, "  %s---%s\n", edge.nodeA, edge.nodeB)
	}
}

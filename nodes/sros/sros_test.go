package sros

import (
	"testing"

	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestComponentSorting(t *testing.T) {
	tests := []struct {
		name     string
		input    []*clabtypes.Component
		expected []string // expected slot order
	}{
		{
			name: "random mix",
			input: []*clabtypes.Component{
				{Slot: "3"},
				{Slot: "b"},
				{Slot: "1"},
				{Slot: "A"},
				{Slot: "2"},
			},
			expected: []string{"A", "b", "1", "2", "3"},
		},
		{
			name: "iom/xcm only",
			input: []*clabtypes.Component{
				{Slot: "3"},
				{Slot: "1"},
				{Slot: "2"},
				{Slot: "10"},
			},
			expected: []string{"1", "2", "3", "10"},
		},
		{
			name: "cpm only",
			input: []*clabtypes.Component{
				{Slot: "b"},
				{Slot: "A"},
			},
			expected: []string{"A", "b"},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			n := &sros{}
			n.Cfg = &clabtypes.NodeConfig{
				Components: c.input,
			}

			n.sortComponents()

			if len(n.Cfg.Components) != len(c.expected) {
				t.Fatalf("expected %d components, got %d", len(c.expected), len(n.Cfg.Components))
			}

			for i, expectedSlot := range c.expected {
				actualSlot := n.Cfg.Components[i].Slot
				if actualSlot != expectedSlot {
					t.Errorf("expected slot %q, got %q", expectedSlot, actualSlot)
				}
			}
		})
	}
}

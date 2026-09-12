package cmd

import (
	"slices"
	"testing"

	clabcore "github.com/srl-labs/containerlab/core"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	"go.uber.org/mock/gomock"
)

func TestPlaceholderNodeKind(t *testing.T) {
	tests := map[clablinks.LinkEndpointType]string{
		clablinks.LinkEndpointTypeHost:     "host",
		clablinks.LinkEndpointTypeBridge:   "bridge",
		clablinks.LinkEndpointTypeBridgeNS: "bridge",
		clablinks.LinkEndpointTypeVeth:     "ext-container",
		"anything-else":                    "ext-container",
	}

	for epType, want := range tests {
		if got := placeholderNodeKind(epType); got != want {
			t.Errorf("placeholderNodeKind(%q) = %q, want %q", epType, got, want)
		}
	}
}

func TestCreatePlaceholderNodes(t *testing.T) {
	const runtimeName = "test"

	ctrl := gomock.NewController(t)
	newLab := func(t *testing.T) *clabcore.CLab {
		t.Helper()

		c, err := clabcore.NewContainerLab()
		if err != nil {
			t.Fatal(err)
		}
		c.Runtimes[runtimeName] = clabmocksmockruntime.NewMockContainerRuntime(ctrl)

		return c
	}

	t.Run("returns only added placeholders", func(t *testing.T) {
		c := newLab(t)
		existingNode := clabmocksmocknodes.NewMockNode(ctrl)
		c.Nodes["n1"] = existingNode

		got, err := createPlaceholderNodes(
			c,
			parsedEndpoint{Node: "n1", Kind: clablinks.LinkEndpointTypeVeth},
			parsedEndpoint{Node: "external", Kind: clablinks.LinkEndpointTypeVeth},
			runtimeName,
		)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"external"}; !slices.Equal(got, want) {
			t.Fatalf("placeholder node names = %v, want %v", got, want)
		}
		if c.Nodes["n1"] != existingNode {
			t.Fatal("existing topology node was replaced")
		}
		if got := c.Nodes["external"].Config().Kind; got != "ext-container" {
			t.Fatalf("placeholder node kind = %q, want %q", got, "ext-container")
		}
	})

	t.Run("deduplicates same-node endpoints", func(t *testing.T) {
		c := newLab(t)

		got, err := createPlaceholderNodes(
			c,
			parsedEndpoint{Node: "external", Iface: "eth1", Kind: clablinks.LinkEndpointTypeVeth},
			parsedEndpoint{Node: "external", Iface: "eth2", Kind: clablinks.LinkEndpointTypeVeth},
			runtimeName,
		)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"external"}; !slices.Equal(got, want) {
			t.Fatalf("placeholder node names = %v, want %v", got, want)
		}
	})
}

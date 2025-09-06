// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	claberrors "github.com/srl-labs/containerlab/errors"
	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	_ "github.com/srl-labs/containerlab/runtime/all"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
	"golang.org/x/exp/slices"
)

// getNodeMap return a map of nodes for testing purpose.
func getNodeMap(mockCtrl *gomock.Controller) map[string]clabnodes.Node {
	// instantiate Mock Node 1
	mockNode1 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode1.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node1",
			Stages:    clabtypes.NewStages(),
		},
	).AnyTimes()

	// instantiate Mock Node 2
	mockNode2 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode2.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node2",
			Stages: &clabtypes.Stages{
				Create: &clabtypes.StageCreate{
					StageBase: clabtypes.StageBase{
						WaitFor: clabtypes.WaitForList{
							&clabtypes.WaitFor{
								Node:  "node1",
								Stage: clabtypes.WaitForCreate,
							},
						},
					},
				},
				CreateLinks: &clabtypes.StageCreateLinks{
					StageBase: clabtypes.StageBase{},
				},
				Configure: &clabtypes.StageConfigure{
					StageBase: clabtypes.StageBase{},
				},
				Healthy: &clabtypes.StageHealthy{
					StageBase: clabtypes.StageBase{},
				},
				Exit: &clabtypes.StageExit{
					StageBase: clabtypes.StageBase{},
				},
			},
		},
	).AnyTimes()

	// instantiate Mock Node 3
	mockNode3 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode3.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			Image:       "alpine:3",
			NetworkMode: "container:node2",
			ShortName:   "node3",
			Stages: &clabtypes.Stages{
				Create: &clabtypes.StageCreate{
					StageBase: clabtypes.StageBase{
						WaitFor: clabtypes.WaitForList{
							&clabtypes.WaitFor{
								Node:  "node1",
								Stage: clabtypes.WaitForCreate,
							},
							&clabtypes.WaitFor{
								Node:  "node2",
								Stage: clabtypes.WaitForCreate,
							},
						},
					},
				},
				CreateLinks: &clabtypes.StageCreateLinks{
					StageBase: clabtypes.StageBase{},
				},
				Configure: &clabtypes.StageConfigure{
					StageBase: clabtypes.StageBase{},
				},
				Healthy: &clabtypes.StageHealthy{
					StageBase: clabtypes.StageBase{},
				},
				Exit: &clabtypes.StageExit{
					StageBase: clabtypes.StageBase{},
				},
			},
		},
	).AnyTimes()

	// instantiate Mock Node 4
	mockNode4 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode4.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.1",
			ShortName:       "node4",
			NetworkMode:     "container:foobar",
			Stages:          clabtypes.NewStages(),
		},
	).AnyTimes()

	// instantiate Mock Node 5
	mockNode5 := clabmocksmocknodes.NewMockNode(mockCtrl)
	mockNode5.EXPECT().Config().Return(
		&clabtypes.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.2",
			ShortName:       "node5",
			Stages: &clabtypes.Stages{
				Create: &clabtypes.StageCreate{
					StageBase: clabtypes.StageBase{
						WaitFor: clabtypes.WaitForList{
							&clabtypes.WaitFor{
								Node:  "node3",
								Stage: clabtypes.WaitForCreate,
							},
							&clabtypes.WaitFor{
								Node:  "node4",
								Stage: clabtypes.WaitForCreate,
							},
						},
					},
				},
				CreateLinks: &clabtypes.StageCreateLinks{
					StageBase: clabtypes.StageBase{},
				},
				Configure: &clabtypes.StageConfigure{
					StageBase: clabtypes.StageBase{},
				},
				Healthy: &clabtypes.StageHealthy{
					StageBase: clabtypes.StageBase{},
				},
				Exit: &clabtypes.StageExit{
					StageBase: clabtypes.StageBase{},
				},
			},
		},
	).AnyTimes()

	// nodeMap used for testing
	nodeMap := map[string]clabnodes.Node{}

	// nodemap is created with the node definition
	for _, x := range []clabnodes.Node{mockNode1, mockNode2, mockNode3, mockNode4, mockNode5} {
		// add node to nodemap
		nodeMap[x.Config().ShortName] = x
		// add node to dependencyManager
	}

	return nodeMap
}

func Test_WaitForExternalNodeDependencies_OK(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// init a ContainerRuntime mock
	crMock := clabmocksmockruntime.NewMockContainerRuntime(mockCtrl)

	// context parameter
	ctx := context.TODO()

	counter := 0
	counterMax := 3
	// setup the container runtime mock
	crMock.EXPECT().GetContainerStatus(ctx, "foobar").DoAndReturn(
		func(_ context.Context, _ string) clabruntime.ContainerStatus {
			counter++
			if counter >= counterMax {
				return clabruntime.Running
			}
			return clabruntime.Stopped
		},
	).Times(counterMax)

	// create a barebone CLab struct
	c := CLab{
		Nodes:             getNodeMap(mockCtrl),
		globalRuntimeName: "mock",
		Runtimes: map[string]clabruntime.ContainerRuntime{
			"mock": crMock,
		},
	}

	// run the check
	c.waitForExternalNodeDependencies(ctx, "node4")

	// check that the function was called "counterMax" times
	if counter != counterMax {
		t.Errorf("expected %q calls to runtime for status. Seen just %q", counterMax, counter)
	}
}

func Test_WaitForExternalNodeDependencies_NoContainerNetworkMode(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// create a barebone CLab struct
	c := CLab{
		Nodes: getNodeMap(mockCtrl),
	}

	// run the check with a node that has no "network-mode: container:<CONTAINERNAME>"
	c.waitForExternalNodeDependencies(context.TODO(), "node5")
	// should simply and quickly return
}

func Test_WaitForExternalNodeDependencies_NodeNonExisting(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// create a barebone CLab struct
	c := CLab{
		Nodes: getNodeMap(mockCtrl),
	}

	// run the check with a node that has no "network-mode: container:<CONTAINERNAME>"
	c.waitForExternalNodeDependencies(context.TODO(), "NonExistingNode")
	// should simply and quickly return
}

func Test_filterClabNodes(t *testing.T) {
	tests := map[string]struct {
		c           *CLab
		nodesFilter []string
		wantNodes   []string
		wantErr     bool
		err         error
	}{
		"two nodes, one filter node": {
			c: &CLab{
				Config: &Config{
					Topology: &clabtypes.Topology{
						Nodes: map[string]*clabtypes.NodeDefinition{
							"node1": {
								Kind: "linux",
							},
							"node2": {
								Kind: "linux",
							},
						},
					},
				},
			},
			nodesFilter: []string{"node1"},
			wantNodes:   []string{"node1"},
			wantErr:     false,
		},
		"one node, empty node filter": {
			c: &CLab{
				Config: &Config{
					Topology: &clabtypes.Topology{
						Nodes: map[string]*clabtypes.NodeDefinition{
							"node1": {
								Kind: "linux",
							},
						},
					},
				},
			},
			nodesFilter: []string{},
			wantNodes:   []string{"node1"},
			wantErr:     false,
		},
		"two nodes, one filter node with a wrong name": {
			c: &CLab{
				Config: &Config{
					Topology: &clabtypes.Topology{
						Nodes: map[string]*clabtypes.NodeDefinition{
							"node1": {
								Kind: "linux",
							},
							"node2": {
								Kind: "linux",
							},
						},
					},
				},
			},
			nodesFilter: []string{"wrongName"},
			wantNodes:   []string{"node1", "node2"},
			wantErr:     true,
			err:         claberrors.ErrIncorrectInput,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.c.filterClabNodes(tt.nodesFilter)
			if (err != nil) != tt.wantErr {
				t.Log("hey", tt.c.Config.Topology.Nodes)
				t.Fatalf("filterClabNodes() error = %v, wantErr %v", err, tt.wantErr)
			} else if !errors.Is(err, tt.err) {
				t.Log("hey", tt.c.Config.Topology.Nodes)
				t.Fatalf("filterClabNodes() error = %v, wantErr %v", err, tt.err)
			}

			filteredNodes := make([]string, 0, len(tt.c.Config.Topology.Nodes))
			for n := range tt.c.Config.Topology.Nodes {
				filteredNodes = append(filteredNodes, n)
			}
			// sort the nodes to make the test deterministic
			slices.Sort(filteredNodes)

			if cmp.Diff(filteredNodes, tt.wantNodes) != "" {
				t.Errorf("filterClabNodes() got = %v, want %v", filteredNodes, tt.wantNodes)
			}
		})
	}
}

func TestWithTopo(t *testing.T) {
	type args struct {
		topoRef string
	}

	tests := []struct {
		name      string
		args      args
		wantError bool
	}{
		{
			name: "empty toporef",
			args: args{
				topoRef: "",
			},
			wantError: true,
		},
		{
			name: "ref single file",
			args: args{
				topoRef: "../lab-examples/srl01/srl01.clab.yml",
			},
			wantError: false,
		},
		{
			name: "no topology in folder",
			args: args{
				topoRef: "../cmd",
			},
			wantError: true,
		},
		{
			name: "single topology in folder",
			args: args{
				topoRef: "../lab-examples/srl01/",
			},
			wantError: false,
		},
		{
			name: "multiple topologies in folder",
			args: args{
				topoRef: "./tests/01-smoke",
			},
			wantError: true,
		},
		{
			name: "non existing folder",
			args: args{
				topoRef: "/someNonExistingFolder",
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := WithTopoPath(tt.args.topoRef, "")

			c, err := NewContainerLab()
			if err != nil {
				t.Error(err)
			}

			err = wt(c)
			if tt.wantError && err == nil {
				t.Errorf("expected error, got non")
			}

			if !tt.wantError && err != nil {
				t.Errorf("got error %v, expected no error", err)
			}
		})
	}
}

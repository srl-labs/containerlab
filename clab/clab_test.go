// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	errs "github.com/srl-labs/containerlab/errors"
	"github.com/srl-labs/containerlab/mocks"
	"github.com/srl-labs/containerlab/mocks/mocknodes"
	"github.com/srl-labs/containerlab/mocks/mockruntime"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	_ "github.com/srl-labs/containerlab/runtime/all"
	"github.com/srl-labs/containerlab/types"
	"golang.org/x/exp/slices"
)

func Test_createNamespaceSharingDependencyOne(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// instantiate a dependencyManager mock
	dm := mocks.NewMockDependencyManager(mockCtrl)

	// retrieve a map of nodes
	nodeMap := getNodeMap(mockCtrl)

	dm.EXPECT().AddDependency("node2", "node3")
	createNamespaceSharingDependency(nodeMap, dm)
}

func Test_createStaticDynamicDependency(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// instantiate a dependencyManager mock
	dm := mocks.NewMockDependencyManager(mockCtrl)

	// retrieve a map of nodes
	nodeMap := getNodeMap(mockCtrl)

	dm.EXPECT().AddDependency("node4", "node1")
	dm.EXPECT().AddDependency("node4", "node2")
	dm.EXPECT().AddDependency("node4", "node3")
	dm.EXPECT().AddDependency("node5", "node1")
	dm.EXPECT().AddDependency("node5", "node2")
	dm.EXPECT().AddDependency("node5", "node3")

	createStaticDynamicDependency(nodeMap, dm)
}

// getNodeMap return a map of nodes for testing purpose.
func getNodeMap(mockCtrl *gomock.Controller) map[string]nodes.Node {
	// instantiate Mock Node 1
	mockNode1 := mocknodes.NewMockNode(mockCtrl)
	mockNode1.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node1",
		},
	).AnyTimes()

	// instantiate Mock Node 2
	mockNode2 := mocknodes.NewMockNode(mockCtrl)
	mockNode2.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node2",
			WaitFor:   []string{"node1"},
		},
	).AnyTimes()

	// instantiate Mock Node 3
	mockNode3 := mocknodes.NewMockNode(mockCtrl)
	mockNode3.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:       "alpine:3",
			NetworkMode: "container:node2",
			ShortName:   "node3",
			WaitFor:     []string{"node1", "node2"},
		},
	).AnyTimes()

	// instantiate Mock Node 4
	mockNode4 := mocknodes.NewMockNode(mockCtrl)
	mockNode4.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.1",
			ShortName:       "node4",
			NetworkMode:     "container:foobar",
		},
	).AnyTimes()

	// instantiate Mock Node 5
	mockNode5 := mocknodes.NewMockNode(mockCtrl)
	mockNode5.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.2",
			ShortName:       "node5",
			WaitFor:         []string{"node3", "node4"},
		},
	).AnyTimes()

	// nodeMap used for testing
	nodeMap := map[string]nodes.Node{}

	// nodemap is created with the node definition
	for _, x := range []nodes.Node{mockNode1, mockNode2, mockNode3, mockNode4, mockNode5} {
		// add node to nodemap
		nodeMap[x.Config().ShortName] = x
		// add node to dependencyManager
	}

	return nodeMap
}

func Test_createWaitForDependency(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// instantiate a dependencyManager mock
	dm := mocks.NewMockDependencyManager(mockCtrl)

	// retrieve a map of nodes
	nodeMap := getNodeMap(mockCtrl)

	dm.EXPECT().AddDependency("node1", "node2")
	dm.EXPECT().AddDependency("node1", "node3")
	dm.EXPECT().AddDependency("node2", "node3")
	dm.EXPECT().AddDependency("node3", "node5")
	dm.EXPECT().AddDependency("node4", "node5")

	err := createWaitForDependency(nodeMap, dm)
	if err != nil {
		t.Error(err)
	}
}

func Test_WaitForExternalNodeDependencies_OK(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// init a ContainerRuntime mock
	crMock := mockruntime.NewMockContainerRuntime(mockCtrl)

	// context parameter
	ctx := context.TODO()

	counter := 0
	counterMax := 3
	// setup the container runtime mock
	crMock.EXPECT().GetContainerStatus(ctx, "foobar").DoAndReturn(
		func(_ context.Context, _ string) runtime.ContainerStatus {
			counter++
			if counter >= counterMax {
				return runtime.Running
			}
			return runtime.Stopped
		},
	).Times(counterMax)

	// create a barebone CLab struct
	c := CLab{
		Nodes:         getNodeMap(mockCtrl),
		globalRuntime: "mock",
		Runtimes: map[string]runtime.ContainerRuntime{
			"mock": crMock,
		},
	}

	// run the check
	c.WaitForExternalNodeDependencies(ctx, "node4")

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
	c.WaitForExternalNodeDependencies(context.TODO(), "node5")
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
	c.WaitForExternalNodeDependencies(context.TODO(), "NonExistingNode")
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
					Topology: &types.Topology{
						Nodes: map[string]*types.NodeDefinition{
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
					Topology: &types.Topology{
						Nodes: map[string]*types.NodeDefinition{
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
					Topology: &types.Topology{
						Nodes: map[string]*types.NodeDefinition{
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
			err:         errs.ErrIncorrectInput,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.c.filterClabNodes(tt.nodesFilter)
			if (err != nil) != tt.wantErr {
				t.Log("hey", tt.c.Config.Topology.Nodes)
				t.Fatalf("filterClabNodes() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if !errors.Is(err, tt.err) {
					t.Log("hey", tt.c.Config.Topology.Nodes)
					t.Fatalf("filterClabNodes() error = %v, wantErr %v", err, tt.err)
				}
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

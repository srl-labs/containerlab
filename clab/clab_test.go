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
	mockNode1 := mocks.NewMockNode(mockCtrl)
	mockNode1.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node1",
		},
	).AnyTimes()

	// instantiate Mock Node 2
	mockNode2 := mocks.NewMockNode(mockCtrl)
	mockNode2.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:     "alpine:3",
			ShortName: "node2",
			WaitFor:   []string{"node1"},
		},
	).AnyTimes()

	// instantiate Mock Node 3
	mockNode3 := mocks.NewMockNode(mockCtrl)
	mockNode3.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:       "alpine:3",
			NetworkMode: "container:node2",
			ShortName:   "node3",
			WaitFor:     []string{"node1", "node2"},
		},
	).AnyTimes()

	// instantiate Mock Node 4
	mockNode4 := mocks.NewMockNode(mockCtrl)
	mockNode4.EXPECT().Config().Return(
		&types.NodeConfig{
			Image:           "alpine:3",
			MgmtIPv4Address: "172.10.10.1",
			ShortName:       "node4",
			NetworkMode:     "container:foobar",
		},
	).AnyTimes()

	// instantiate Mock Node 5
	mockNode5 := mocks.NewMockNode(mockCtrl)
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
	crMock := mocks.NewMockContainerRuntime(mockCtrl)

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
		wantLinks   [][]string
		wantErr     bool
		err         error
	}{
		"two nodes, no links, one filter node": {
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
			wantLinks:   [][]string{},
			wantErr:     false,
		},
		"one node, no links, empty node filter": {
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
			wantLinks:   [][]string{},
			wantErr:     false,
		},
		"two nodes, one link between them, one filter node": {
			c: &CLab{
				Links: map[int]*types.Link{
					0: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node1",
							},
							EndpointName: "eth1",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth2",
						},
					},
				},
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
			wantLinks:   [][]string{},
			wantErr:     false,
		},
		"two nodes, one link between them, no filter": {
			c: &CLab{
				Links: map[int]*types.Link{
					0: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node1",
							},
							EndpointName: "eth1",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth1",
						},
					},
				},
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
			nodesFilter: []string{},
			wantNodes:   []string{"node1", "node2"},
			wantLinks:   [][]string{{"node1:eth1", "node2:eth1"}},
			wantErr:     false,
		},
		"three nodes, two links, two nodes in the filter": {
			c: &CLab{
				Links: map[int]*types.Link{
					0: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node1",
							},
							EndpointName: "eth1",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth1",
						},
					},
					1: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth2",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node3",
							},
							EndpointName: "eth2",
						},
					},
				},
				Config: &Config{
					Topology: &types.Topology{
						Nodes: map[string]*types.NodeDefinition{
							"node1": {
								Kind: "linux",
							},
							"node2": {
								Kind: "linux",
							},
							"node3": {
								Kind: "linux",
							},
						},
					},
				},
			},
			nodesFilter: []string{"node1", "node2"},
			wantNodes:   []string{"node1", "node2"},
			wantLinks:   [][]string{{"node1:eth1", "node2:eth1"}},
			wantErr:     false,
		},
		"three nodes, two links, one nodes in the filter": {
			c: &CLab{
				Links: map[int]*types.Link{
					0: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node1",
							},
							EndpointName: "eth1",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth1",
						},
					},
					1: {
						A: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node2",
							},
							EndpointName: "eth2",
						},
						B: &types.Endpoint{
							Node: &types.NodeConfig{
								ShortName: "node3",
							},
							EndpointName: "eth2",
						},
					},
				},
				Config: &Config{
					Topology: &types.Topology{
						Nodes: map[string]*types.NodeDefinition{
							"node1": {
								Kind: "linux",
							},
							"node2": {
								Kind: "linux",
							},
							"node3": {
								Kind: "linux",
							},
						},
					},
				},
			},
			nodesFilter: []string{"node1"},
			wantNodes:   []string{"node1"},
			wantLinks:   [][]string{},
			wantErr:     false,
		},
		"two nodes, no links, one filter node with a wrong name": {
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
			wantLinks:   [][]string{},
			wantErr:     true,
			err:         errs.ErrIncorrectInput,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := filterClabNodes(tt.c, tt.nodesFilter)
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

			filteredLinks := make([][]string, 0, len(tt.c.Links))
			for _, l := range tt.c.Links {
				filteredLinks = append(filteredLinks, []string{l.A.String(), l.B.String()})
			}

			if cmp.Diff(filteredNodes, tt.wantNodes) != "" {
				t.Errorf("filterClabNodes() got = %v, want %v", filteredNodes, tt.wantNodes)
			}

			if cmp.Diff(filteredLinks, tt.wantLinks) != "" {
				t.Errorf("filterClabNodes() got = %v, want %v", filteredLinks, tt.wantLinks)
			}
		})
	}
}

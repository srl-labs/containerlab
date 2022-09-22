// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package clab

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/srl-labs/containerlab/mocks"
	"github.com/srl-labs/containerlab/nodes"
	_ "github.com/srl-labs/containerlab/nodes/all"
	"github.com/srl-labs/containerlab/runtime"
	_ "github.com/srl-labs/containerlab/runtime/all"
	"github.com/srl-labs/containerlab/types"
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

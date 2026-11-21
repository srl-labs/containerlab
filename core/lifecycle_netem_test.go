package core

import (
	"context"
	"errors"
	"testing"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func newLifecycleNetemTestCLab(nodes map[string]clabnodes.Node) *CLab {
	return &CLab{
		Config: &Config{
			Mgmt:     new(clabtypes.MgmtNet),
			Topology: clabtypes.NewTopology(),
		},
		Nodes: nodes,
	}
}

func TestStartNodesSelectsNodesAndSkipsFailedNodeEndpoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	startErr := errors.New("start failed")
	failed := clabmocksmocknodes.NewMockNode(ctrl)
	other := clabmocksmocknodes.NewMockNode(ctrl)
	failed.EXPECT().Start(gomock.Any()).Return(startErr)

	c := newLifecycleNetemTestCLab(map[string]clabnodes.Node{
		"failed": failed,
		"other":  other,
	})
	if err := c.StartNodes(context.Background(), []string{"failed"}); !errors.Is(err, startErr) {
		t.Fatalf("StartNodes() error = %v, want %v", err, startErr)
	}
}

func TestRestartNodesCollectsOnlySuccessfullyStartedNodeEndpoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	startErr := errors.New("start failed")
	started := clabmocksmocknodes.NewMockNode(ctrl)
	failed := clabmocksmocknodes.NewMockNode(ctrl)
	other := clabmocksmocknodes.NewMockNode(ctrl)
	started.EXPECT().Stop(gomock.Any()).Return(nil)
	started.EXPECT().Start(gomock.Any()).Return(nil)
	started.EXPECT().GetEndpoints().Return(nil)
	failed.EXPECT().Stop(gomock.Any()).Return(nil)
	failed.EXPECT().Start(gomock.Any()).Return(startErr)

	c := newLifecycleNetemTestCLab(map[string]clabnodes.Node{
		"started": started,
		"failed":  failed,
		"other":   other,
	})
	if err := c.RestartNodes(
		context.Background(),
		[]string{"started", "failed", "other"},
	); !errors.Is(err, startErr) {
		t.Fatalf("RestartNodes() error = %v, want %v", err, startErr)
	}
}

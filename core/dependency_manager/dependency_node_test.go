package dependency_manager

import (
	"context"
	"testing"
	"time"

	clabmocksmocknodes "github.com/srl-labs/containerlab/mocks/mocknodes"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

// Test_EnterStage_HonorsContextCancellation verifies that EnterStage returns
// when the context is cancelled instead of blocking forever on the stage
// waitgroup. Without this, a node waiting on a dependency that never completes
// (e.g. a crashed container after Ctrl-C) keeps the deploy hanging until SIGQUIT
// (issue #3162).
func Test_EnterStage_HonorsContextCancellation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	newNode := func(name string) *DependencyNode {
		mn := clabmocksmocknodes.NewMockNode(mockCtrl)
		mn.EXPECT().Config().Return(&clabtypes.NodeConfig{
			ShortName: name,
			Stages:    clabtypes.NewStages(),
		}).AnyTimes()
		mn.EXPECT().GetShortName().Return(name).AnyTimes()

		return NewDependencyNode(mn)
	}

	waiter := newNode("waiter")
	dependee := newNode("dependee")

	// waiter must wait for dependee's healthy stage before it can enter its own
	// configure stage. This bumps waiter.stageWG[WaitForConfigure] to 1, so
	// EnterStage(WaitForConfigure) blocks until dependee signals Done (which never
	// happens here) or the context is cancelled.
	if err := dependee.AddDepender(
		clabtypes.WaitForConfigure, waiter, clabtypes.WaitForHealthy,
	); err != nil {
		t.Fatalf("AddDepender failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		waiter.EnterStage(ctx, clabtypes.WaitForConfigure)
		close(done)
	}()

	// the dependency is never satisfied; cancelling the context must unblock it.
	cancel()

	select {
	case <-done:
		// EnterStage returned after cancellation, as expected.
	case <-time.After(5 * time.Second):
		t.Fatal("EnterStage did not return after context cancellation; " +
			"the stage wait is not honoring ctx.Done()")
	}
}

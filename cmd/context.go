package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
)

const (
	// maxCancelledDestroyTimeout bounds the rollback of a cancelled deployment on local
	// runtimes, where removing what was created is a local and fast operation.
	maxCancelledDestroyTimeout = 20 * time.Second
)

var (
	// panics when closed twice, this way there can only be one signal handled context
	onlyOneSignalHandler = make(chan struct{}) //nolint: gochecknoglobals

	// cancellationSignaled is closed once a SIGINT or SIGTERM has been received, letting
	// commands tell a user-requested cancellation apart from an ordinary failure.
	cancellationSignaled = make(chan struct{}) //nolint: gochecknoglobals
)

// SignalHandledContext returns a context that will be canceled if a SIGINT or SIGTERM is
// received. Rolling back whatever the canceled operation had already created is the job of
// the command that owns it, so that its output stays ordered and the rollback cannot race
// with a deployment that is still unwinding. This handler only guarantees that the process
// still ends when there is nothing to roll back, or when the user signals a second time.
func SignalHandledContext() (context.Context, context.CancelFunc) {
	// panics when called twice, this way there can only be one signal handled context
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 2) //nolint:mnd

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs

		log.Errorf("received signal %q, canceling context", sig)

		close(cancellationSignaled)

		cancel()

		options := GetOptions()

		if !options.Global.CleanOnCancel.Load() {
			log.Debug("clean on cancel is not true, exiting")

			os.Exit(1)
		}

		log.Info("Cleaning up the cancelled deployment, press Ctrl+C again to abort")

		// The command performs the cleanup itself; exiting here would cut it short. A
		// cleanup takes as long as the runtime needs to remove what was already created,
		// so it is not put on a timer -- a second signal is the escape hatch instead.
		sig = <-sigs

		log.Errorf("received signal %q during cleanup, exiting", sig)

		os.Exit(1)
	}()

	return ctx, cancel
}

// cancellationRequested reports whether a SIGINT or SIGTERM has been received.
func cancellationRequested() bool {
	select {
	case <-cancellationSignaled:
		return true
	default:
		return false
	}
}

// destroyCancelledDeploy removes the lab that a cancelled deployment was in the middle of
// creating. It runs on a fresh context because the command context is canceled by then.
func destroyCancelledDeploy(o *Options) {
	destroyCtx, destroyCancel := context.WithTimeout(
		context.Background(),
		cancelledDestroyTimeout(o),
	)
	defer destroyCancel()

	// destroyFn requires a cobra.Command but only needs the ctx from it
	destroyCmd := &cobra.Command{}
	destroyCmd.SetContext(destroyCtx)

	err := destroyFn(destroyCmd, o)
	if err != nil {
		log.Errorf("failed destroying lab after cancellation signal: %v", err)
	}
}

// cancelledDestroyTimeout bounds the rollback of a cancelled deployment. Controller-driven
// runtimes delete remote resources and wait for the cluster to converge on the deletion,
// which legitimately outlasts the bound that is generous for a local teardown.
func cancelledDestroyTimeout(o *Options) time.Duration {
	if clablabruntime.IsLabRuntimeName(o.Global.Runtime) {
		return o.Global.Timeout
	}

	return maxCancelledDestroyTimeout
}

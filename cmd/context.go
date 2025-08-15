package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

var onlyOneSignalHandler = make(chan struct{}) //nolint: gochecknoglobals

// SignalHandledContext returns a context that will be canceled if a SIGINT or SIGTERM is
// received.
func SignalHandledContext() (context.Context, context.CancelFunc) {
	// panics when called twice, this way there can only be one signal handled context
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 2) //nolint:mnd

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Errorf("received signal %q, canceling context and cleaning deployment...", sig)

		cancel()

		destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer destroyCancel()

		// destroyFn requires a cobra.Command but only needs the ctx from it
		destroyCmd := &cobra.Command{}
		destroyCmd.SetContext(destroyCtx)

		err := destroyFn(destroyCmd, GetOptions())
		if err != nil {
			log.Errorf("failed destroying lab after cancellation signal: %v", err)
		}

		os.Exit(1)
	}()

	return ctx, cancel
}

package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
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
		log.Infof("received signal '%s', canceling context", sig)

		cancel()

		sig = <-sigs
		log.Infof("received signal '%s', exiting program", sig)

		os.Exit(1)
	}()

	return ctx, cancel
}

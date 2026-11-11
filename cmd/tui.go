// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	clabinternaltui "github.com/srl-labs/containerlab/internal/tui"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuibackendfake "github.com/srl-labs/containerlab/internal/tui/backend/fake"
	clabinternaltuibackendlocal "github.com/srl-labs/containerlab/internal/tui/backend/local"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

const (
	tuiCommandName = "tui"
	// noTUIEnv keeps a bare `containerlab` from opening the TUI, printing the help
	// instead as it always did.
	noTUIEnv = envPrefix + "_NO_TUI"
)

func tuiCmd(o *Options) (*cobra.Command, error) {
	c := &cobra.Command{
		Use:   tuiCommandName,
		Short: "open the full-screen terminal UI",
		Long: `Open a full-screen terminal UI to watch and operate labs.

It lists the labs, nodes, interfaces and links with their live state and traffic, draws
the topology of a lab, and runs the usual operations from the keyboard: deploy, destroy,
shell and SSH sessions, logs, link impairments, packet captures and more. Press ? inside
the UI for the list of keys.

Operations that change a lab run the corresponding containerlab command in the background
and need the same privileges it does. Without them the UI still works for watching.

With a lab runtime, ie. --runtime c9s, the UI shows the labs of the Kubernetes cluster
instead of the ones of this host.`,
		Args: cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			return tuiFn(cobraCmd.Context(), o)
		},
	}

	c.Flags().StringVar(
		&o.TUI.Theme,
		"theme",
		o.TUI.Theme,
		"color theme; one of ["+strings.Join(clabinternaltuitheme.Names(), ", ")+"]",
	)
	c.Flags().BoolVar(
		&o.TUI.NoMouse,
		"no-mouse",
		o.TUI.NoMouse,
		"do not capture the mouse",
	)
	c.Flags().BoolVar(
		&o.TUI.ReadOnly,
		"read-only",
		o.TUI.ReadOnly,
		"only watch: disable the operations that change labs",
	)
	c.Flags().StringSliceVar(
		&o.TUI.LabsDirs,
		"labs-dir",
		o.TUI.LabsDirs,
		"additional directory to look for topology files in (can be specified multiple times)",
	)
	c.Flags().BoolVar(
		&o.TUI.Demo,
		"demo",
		o.TUI.Demo,
		"show simulated labs instead of the real ones; needs neither a container runtime nor root",
	)

	return c, nil
}

// isInteractiveTerminal reports whether the process talks to a terminal a full-screen UI
// can run in.
func isInteractiveTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) &&
		os.Getenv("TERM") != "dumb"
}

// bareCommandOpensTUI reports whether running containerlab without a command should open
// the TUI rather than print the help.
func bareCommandOpensTUI() bool {
	return isInteractiveTerminal() && !noTUI(os.Getenv(noTUIEnv))
}

// noTUI reports whether the value of the variable that turns the TUI off does so. A
// boolean is taken for what it says, any other value that is set turns the TUI off.
func noTUI(value string) bool {
	if value == "" {
		return false
	}

	off, err := strconv.ParseBool(value)

	return err != nil || off
}

// rootFn runs when containerlab is called without a command.
func rootFn(cobraCmd *cobra.Command, o *Options) error {
	if !bareCommandOpensTUI() {
		return cobraCmd.Help()
	}

	// The environment variables of the tui command apply here as well.
	if tuiCommand, _, err := cobraCmd.Root().
		Find([]string{tuiCommandName}); err == nil &&
		v != nil {
		updateOptionsFromViper(tuiCommand, o)
	}

	return tuiFn(cobraCmd.Context(), o)
}

func tuiFn(ctx context.Context, o *Options) error {
	if !isInteractiveTerminal() {
		return errors.New("the TUI needs a terminal; use `containerlab inspect --all` " +
			"to list labs from a script")
	}

	be, err := newTUIBackend(o)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var running atomic.Bool

	restoreSignals := tuiSignals(cancel, running.Load)
	defer restoreSignals()

	restoreLog := silenceLogger()
	defer restoreLog()

	err = clabinternaltui.Run(ctx, be, clabinternaltui.Options{
		Theme:   o.TUI.Theme,
		Lab:     o.Global.TopologyName,
		NoMouse: o.TUI.NoMouse,
		Running: running.Store,
	})
	if err != nil {
		return fmt.Errorf("the TUI stopped: %w", err)
	}

	return nil
}

// newTUIBackend creates the data source of the TUI: the simulated labs of the demo mode,
// the container runtime of this host, or the lab runtime that was asked for.
func newTUIBackend(o *Options) (clabinternaltuibackend.Backend, error) {
	if o.TUI.Demo {
		return clabinternaltuibackendfake.New(clabinternaltuibackendfake.Options{
			Live: true,
			Seed: uint64(time.Now().UnixNano()), //nolint:gosec // any seed will do
		}), nil
	}

	return clabinternaltuibackendlocal.New(clabinternaltuibackendlocal.Options{
		Runtime:   o.Global.Runtime,
		Namespace: o.Global.Namespace,
		Timeout:   o.Global.Timeout,
		LabsDirs:  o.TUI.LabsDirs,
		ReadOnly:  o.TUI.ReadOnly,
		Debug:     o.Global.DebugCount > 0,
	})
}

// tuiSignals takes over the handling of termination signals while the TUI runs. The
// regular handler ends the process on the spot, which would leave the terminal in the
// state the UI put it in. running tells whether the UI owns the terminal. It returns a
// function that hands the signals back.
func tuiSignals(quit context.CancelFunc, running func() bool) (restore func()) {
	handle := func(sig os.Signal) {
		// While the UI owns the terminal, an interrupt can only come from outside of it,
		// where ctrl+c is a key like any other: it is the terminal signaling the session
		// a node shell runs in. That session deals with it, the UI carries on. Before
		// and after, ie. while the labs are listed for the first time, it ends the UI.
		if sig == syscall.SIGINT && running() {
			return
		}

		quit()
	}

	signalDelegate.Store(&handle)

	// A closed terminal ends the UI like a termination request does.
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)

	done := make(chan struct{})

	go func() {
		select {
		case <-hangup:
			quit()
		case <-done:
		}
	}()

	return func() {
		close(done)
		signal.Stop(hangup)
		signalDelegate.Store(nil)
	}
}

// silenceLogger keeps log output off the screen while the TUI owns it. Whatever the
// packages used in-process log would otherwise be written across the UI. It returns a
// function that restores the logger.
func silenceLogger() (restore func()) {
	log.SetOutput(io.Discard)

	return func() { log.SetOutput(os.Stderr) }
}

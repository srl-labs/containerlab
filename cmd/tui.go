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
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	clabinternaltui "github.com/srl-labs/containerlab/internal/tui"
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
	return isInteractiveTerminal() && os.Getenv(noTUIEnv) == ""
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

	if _, err := clabinternaltuitheme.New(o.TUI.Theme, true); err != nil {
		return err
	}

	be, err := newTUIBackend(o)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	restoreSignals := tuiSignals(cancel)
	defer restoreSignals()

	restoreLog := silenceLogger()
	defer restoreLog()

	err = clabinternaltui.Run(ctx, be, clabinternaltui.Options{
		Theme:   o.TUI.Theme,
		Lab:     o.Global.TopologyName,
		NoMouse: o.TUI.NoMouse,
	})
	if err != nil {
		return fmt.Errorf("the TUI stopped: %w", err)
	}

	return nil
}

// tuiSignals takes over the handling of termination signals while the TUI runs. The
// regular handler ends the process on the spot, which would leave the terminal in the
// state the UI put it in. It returns a function that hands the signals back.
func tuiSignals(quit context.CancelFunc) (restore func()) {
	handle := func(sig os.Signal) {
		// An interrupt can only come from outside the UI, where ctrl+c is a key like
		// any other: it is the terminal signaling the session a node shell runs in.
		// That session deals with it, the UI carries on.
		if sig == syscall.SIGINT {
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

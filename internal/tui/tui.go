// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package tui implements the full-screen terminal UI of containerlab: live tables of labs,
// nodes, interfaces and links, a topology map, and the actions to operate them.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
	clabinternaltuigfx "github.com/srl-labs/containerlab/internal/tui/gfx"
	clabinternaltuitheme "github.com/srl-labs/containerlab/internal/tui/theme"
)

// Options configures the TUI.
type Options struct {
	// Theme is the name of the color theme, empty to follow the terminal.
	Theme string
	// Lab limits the view to one lab from the start.
	Lab string
	// NoMouse turns mouse support off.
	NoMouse bool
	// Now is the clock, time.Now when nil.
	Now func() time.Time
	// Home is the user's home directory, used to shorten paths.
	Home string
	// Input and Output replace the terminal, for tests.
	Input  io.Reader
	Output io.Writer
	// Running, when set, is told true once the UI takes the terminal over, and false once
	// it handed it back.
	Running func(bool)
	// Graphics tells whether to draw the pixel layer on terminals that speak the kitty
	// graphics protocol: GraphicsAuto, GraphicsOn or GraphicsOff. Empty means auto.
	Graphics string

	// pixels and screen are the output the pixel layer is sent through, set by Run.
	pixels *clabinternaltuigfx.Output
	screen *clabinternaltuigfx.Screen
}

// Run starts the backend and runs the TUI until the user quits or ctx is done.
func Run(ctx context.Context, be clabinternaltuibackend.Backend, opts Options) error {
	if _, err := clabinternaltuitheme.New(opts.Theme, true); err != nil {
		return err
	}

	switch opts.Graphics {
	case "":
		opts.Graphics = GraphicsAuto
	case GraphicsAuto, GraphicsOn, GraphicsOff:
	default:
		return fmt.Errorf("unknown graphics mode %q, available modes: %s",
			opts.Graphics, strings.Join(GraphicsModes(), ", "))
	}

	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}

	if opts.Running == nil {
		opts.Running = func(bool) {}
	}

	if err := be.Start(ctx); err != nil {
		if ctx.Err() != nil {
			// The user stopped the UI while the labs were listed for the first time.
			return nil
		}

		return err
	}

	defer closeBackend(be, opts.Output)

	// The caller owns the handling of signals and ends the UI through the context.
	programOpts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithoutSignalHandler()}
	if opts.Input != nil {
		programOpts = append(programOpts, tea.WithInput(opts.Input))
	}

	if opts.Output != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Output))
	} else if opts.Graphics != GraphicsOff {
		// The pixel layer travels with the frames of text: see gfx.Output.
		opts.screen = clabinternaltuigfx.NewScreen()
		opts.pixels = clabinternaltuigfx.NewOutput(os.Stdout, opts.screen)
		programOpts = append(programOpts, tea.WithOutput(opts.pixels))
	}

	opts.Running(true)
	_, err := tea.NewProgram(New(be, opts), programOpts...).Run()
	opts.Running(false)

	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		// The context ending is a regular way to stop, not a failure.
		return nil
	}

	return err
}

// closeBackend closes the backend once the UI handed the terminal back. Closing waits for
// the operations that are still running to stop, which takes a while for one that cleans
// up after itself, so the user is told what is waited for. out replaces the standard error
// output, for tests.
func closeBackend(be clabinternaltuibackend.Backend, out io.Writer) {
	if out == nil {
		out = os.Stderr
	}

	var running []string

	jobs := be.Jobs()
	for i := range jobs {
		if !jobs[i].State.Done() {
			running = append(running, jobs[i].Title)
		}
	}

	if len(running) > 0 {
		_, _ = fmt.Fprintf(out, "Waiting for %d operation(s) to stop: %s\n",
			len(running), strings.Join(running, ", "))
	}

	be.Close()
}

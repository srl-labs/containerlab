// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package tui implements the full-screen terminal UI of containerlab: live tables of labs,
// nodes, interfaces and links, a topology map, and the actions to operate them.
package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	clabinternaltuibackend "github.com/srl-labs/containerlab/internal/tui/backend"
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
}

// Run starts the backend and runs the TUI until the user quits or ctx is done.
func Run(ctx context.Context, be clabinternaltuibackend.Backend, opts Options) error {
	if _, err := clabinternaltuitheme.New(opts.Theme, true); err != nil {
		return err
	}

	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}

	if err := be.Start(ctx); err != nil {
		return err
	}
	defer be.Close()

	// The caller owns the handling of signals and ends the UI through the context.
	programOpts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithoutSignalHandler()}
	if opts.Input != nil {
		programOpts = append(programOpts, tea.WithInput(opts.Input))
	}

	if opts.Output != nil {
		programOpts = append(programOpts, tea.WithOutput(opts.Output))
	}

	_, err := tea.NewProgram(New(be, opts), programOpts...).Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		// The context ending is a regular way to stop, not a failure.
		return nil
	}

	return err
}

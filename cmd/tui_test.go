// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"bytes"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestTUICommandIsRegistered(t *testing.T) {
	optionsInstance = nil

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	tui, _, err := root.Find([]string{tuiCommandName})
	if err != nil || tui.Name() != tuiCommandName {
		t.Fatalf("the tui command is not registered: %v", err)
	}

	for _, flag := range []string{"theme", "no-mouse", "read-only", "labs-dir", "demo", "graphics"} {
		if tui.Flags().Lookup(flag) == nil {
			t.Errorf("the tui command has no --%s flag", flag)
		}
	}
}

func TestUnknownCommandIsStillAnError(t *testing.T) {
	optionsInstance = nil

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer

	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"no-such-command"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected an unknown command error, got %v", err)
	}
}

func TestTUINeedsATerminal(t *testing.T) {
	optionsInstance = nil
	o := GetOptions()
	o.TUI.Demo = true

	err := tuiFn(t.Context(), o)
	if err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Errorf("expected the TUI to refuse to start without a terminal, got %v", err)
	}
}

func TestTUIIsOfferedForLabRuntimes(t *testing.T) {
	optionsInstance = nil

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	tui, _, err := root.Find([]string{tuiCommandName})
	if err != nil {
		t.Fatal(err)
	}

	if err := checkLabRuntimeCommandSupport(tui, "c9s"); err != nil {
		t.Errorf("the tui command is rejected with a lab runtime: %v", err)
	}
}

// TestAwaitSignalHonorsDelegate checks that a delegate takes the signals over, and that
// they reach the regular handling again once it is gone.
func TestAwaitSignalHonorsDelegate(t *testing.T) {
	sigs := make(chan os.Signal, 2)
	taken := make(chan os.Signal, 1)

	delegate := func(sig os.Signal) { taken <- sig }
	signalDelegate.Store(&delegate)

	defer signalDelegate.Store(nil)

	returned := make(chan os.Signal, 1)

	go func() { returned <- awaitSignal(sigs) }()

	sigs <- syscall.SIGTERM

	select {
	case sig := <-taken:
		if sig != syscall.SIGTERM {
			t.Errorf("the delegate got %v", sig)
		}
	case <-time.After(time.Second):
		t.Fatal("the delegate did not get the signal")
	}

	select {
	case sig := <-returned:
		t.Fatalf("a signal taken by the delegate was also returned: %v", sig)
	case <-time.After(50 * time.Millisecond):
	}

	signalDelegate.Store(nil)

	sigs <- syscall.SIGINT

	select {
	case sig := <-returned:
		if sig != syscall.SIGINT {
			t.Errorf("got %v, want the interrupt", sig)
		}
	case <-time.After(time.Second):
		t.Fatal("the signal was not returned once the delegate was gone")
	}
}

// TestTUISignalsInterruptBeforeTheUIRuns checks that an interrupt ends the TUI until the UI
// owns the terminal, and is left to the sessions started from it afterwards.
func TestTUISignalsInterruptBeforeTheUIRuns(t *testing.T) {
	var running atomic.Bool

	quits := 0
	restore := tuiSignals(func() { quits++ }, running.Load)

	defer restore()

	handle := *signalDelegate.Load()

	handle(syscall.SIGINT)

	if quits != 1 {
		t.Fatalf("an interrupt while the labs are listed did not end the TUI")
	}

	running.Store(true)
	handle(syscall.SIGINT)

	if quits != 1 {
		t.Errorf("an interrupt while the UI runs ended it")
	}

	handle(syscall.SIGTERM)

	if quits != 2 {
		t.Errorf("a termination request while the UI runs did not end it")
	}
}

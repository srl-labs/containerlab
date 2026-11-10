// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package cmd

import (
	"bytes"
	"os"
	"strings"
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

	for _, flag := range []string{"theme", "no-mouse", "read-only", "labs-dir", "demo"} {
		if tui.Flags().Lookup(flag) == nil {
			t.Errorf("the tui command has no --%s flag", flag)
		}
	}
}

// TestBareCommandPrintsHelpWithoutTerminal runs containerlab without a command. Tests do
// not run in a terminal, so the help must come out, as it did before the TUI existed.
func TestBareCommandPrintsHelpWithoutTerminal(t *testing.T) {
	optionsInstance = nil

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer

	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(nil)

	if err := root.Execute(); err != nil {
		t.Fatalf("running without a command failed: %v", err)
	}

	for _, want := range []string{"Usage:", "Available Commands:", "deploy", tuiCommandName} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the help does not mention %q:\n%s", want, out.String())
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

func TestTUIIsNotOfferedForLabRuntimes(t *testing.T) {
	optionsInstance = nil

	root, err := Entrypoint()
	if err != nil {
		t.Fatal(err)
	}

	tui, _, err := root.Find([]string{tuiCommandName})
	if err != nil {
		t.Fatal(err)
	}

	if err := checkLabRuntimeCommandSupport(tui, "c9s"); err == nil {
		t.Error("the tui command should be rejected with a lab runtime")
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

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package ceos

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabtypes "github.com/srl-labs/containerlab/types"
)

func TestCeosLinkApplyMode(t *testing.T) {
	if got := (&ceos{}).LinkApplyMode(context.Background()); got != clabnodes.LinkApplyModeRestart {
		t.Fatalf("LinkApplyMode() = %q, want %q", got, clabnodes.LinkApplyModeRestart)
	}
}

func TestCeosPostDeployBuildsRuntimeExecCommand(t *testing.T) {
	node := newTestCEOSNode()
	node.Endpoints = []clablinks.Endpoint{
		&clablinks.EndpointGeneric{
			IfaceName: "eth1",
			IPv4:      netip.MustParsePrefix("192.0.2.1/31"),
			IPv6:      netip.MustParsePrefix("2001:db8::1/127"),
		},
	}

	var gotCmd string
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			gotCmd = execCmd.GetCmdString()
			return execResult(execCmd, 0, "", ""), nil
		},
		func(time.Duration) {},
	)
	defer restore()

	if err := node.ceosPostDeploy(context.Background()); err != nil {
		t.Fatalf("ceosPostDeploy() unexpected error: %v", err)
	}

	wantSnippets := []string{
		"/bin/bash -lc",
		"Cli -p 15 --abort-on-error -c",
		"configure terminal",
		"interface Management0",
		"ip address 172.20.20.2/24",
		"ipv6 address 2001:db8:1::2/64",
		"interface eth1",
		"no switchport",
		"ip address 192.0.2.1/31",
		"ipv6 address 2001:db8::1/127",
		"end",
		"write memory",
	}

	for _, want := range wantSnippets {
		if !strings.Contains(gotCmd, want) {
			t.Fatalf("exec command missing %q\ncommand: %s", want, gotCmd)
		}
	}
}

func TestCeosPostDeployRetriesUntilSuccess(t *testing.T) {
	node := newTestCEOSNode()

	var calls, sleeps int
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			calls++
			if calls == 1 {
				return execResult(execCmd, 1, "", "Cli not ready"), nil
			}
			return execResult(execCmd, 0, "", ""), nil
		},
		func(time.Duration) { sleeps++ },
	)
	defer restore()

	if err := node.ceosPostDeploy(context.Background()); err != nil {
		t.Fatalf("ceosPostDeploy() unexpected error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("exec calls = %d, want 2", calls)
	}
	if sleeps != 1 {
		t.Fatalf("sleep calls = %d, want 1", sleeps)
	}
}

func TestCeosPostDeployReturnsExecErrorAfterRetries(t *testing.T) {
	node := newTestCEOSNode()
	wantErr := errors.New("runtime unavailable")

	var calls, sleeps int
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, _ *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			calls++
			return nil, wantErr
		},
		func(time.Duration) { sleeps++ },
	)
	defer restore()

	err := node.ceosPostDeploy(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("ceosPostDeploy() error = %v, want %v", err, wantErr)
	}
	if calls != 60 {
		t.Fatalf("exec calls = %d, want 60", calls)
	}
	if sleeps != 60 {
		t.Fatalf("sleep calls = %d, want 60", sleeps)
	}
}

func TestCeosPostDeployReturnsCLIErrorAfterRetries(t *testing.T) {
	node := newTestCEOSNode()

	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			return execResult(execCmd, 1, "partial output", "syntax error"), nil
		},
		func(time.Duration) {},
	)
	defer restore()

	err := node.ceosPostDeploy(context.Background())
	if err == nil {
		t.Fatal("ceosPostDeploy() error = nil, want non-nil")
	}

	for _, want := range []string{
		"failed CLI configuration",
		"rc=1",
		`stdout="partial output"`,
		`stderr="syntax error"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func newTestCEOSNode() *ceos {
	node := &ceos{}
	node.DefaultNode = *clabnodes.NewDefaultNode(node)
	node.Cfg = &clabtypes.NodeConfig{
		ShortName:            "ceos1",
		LongName:             "clab-test-ceos1",
		MgmtIntf:             "Management0",
		MgmtIPv4Address:      "172.20.20.2",
		MgmtIPv4PrefixLength: 24,
		MgmtIPv6Address:      "2001:db8:1::2",
		MgmtIPv6PrefixLength: 64,
	}

	return node
}

func execResult(execCmd *clabexec.ExecCmd, rc int, stdout, stderr string) *clabexec.ExecResult {
	res := clabexec.NewExecResult(execCmd)
	res.SetReturnCode(rc)
	res.SetStdOut([]byte(stdout))
	res.SetStdErr([]byte(stderr))
	return res
}

func stubCeosPostDeploy(
	execFn func(*ceos, context.Context, *clabexec.ExecCmd) (*clabexec.ExecResult, error),
	sleepFn func(time.Duration),
) func() {
	origExec := ceosPostDeployExec
	origSleep := ceosPostDeploySleep

	ceosPostDeployExec = execFn
	ceosPostDeploySleep = sleepFn

	return func() {
		ceosPostDeployExec = origExec
		ceosPostDeploySleep = origSleep
	}
}

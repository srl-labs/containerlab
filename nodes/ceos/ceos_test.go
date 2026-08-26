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
		clablinks.NewEndpointVeth(&clablinks.EndpointGeneric{
			IfaceName: "eth1",
			IPv4:      netip.MustParsePrefix("192.0.2.1/31"),
			IPv6:      netip.MustParsePrefix("2001:db8::1/127"),
		}),
	}

	var gotCmd []string
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			gotCmd = execCmd.GetCmd()
			return execResult(execCmd, 0, "", ""), nil
		},
		func(time.Duration) {},
	)
	defer restore()

	if err := node.ceosPostDeploy(context.Background()); err != nil {
		t.Fatalf("ceosPostDeploy() unexpected error: %v", err)
	}

	wantArgs := []string{"Cli", "-p", "15", "--abort-on-error", "-c"}
	if len(gotCmd) != len(wantArgs)+1 {
		t.Fatalf("exec command has %d arguments, want %d: %q", len(gotCmd), len(wantArgs)+1, gotCmd)
	}
	for i, want := range wantArgs {
		if gotCmd[i] != want {
			t.Fatalf("exec command argument %d = %q, want %q", i, gotCmd[i], want)
		}
	}

	wantConfigSnippets := []string{
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

	config := gotCmd[len(wantArgs)]
	for _, want := range wantConfigSnippets {
		if !strings.Contains(config, want) {
			t.Fatalf("CLI configuration missing %q\nconfiguration: %s", want, config)
		}
	}
}

func TestCeosPostDeployWaitsForCLIThenConfiguresOnce(t *testing.T) {
	node := newTestCEOSNode()

	var calls, configCalls, sleeps int
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			calls++
			if isCeosReadinessCommand(execCmd) && calls == 1 {
				return execResult(execCmd, 1, "", "Cli not ready"), nil
			}
			if !isCeosReadinessCommand(execCmd) {
				configCalls++
			}
			return execResult(execCmd, 0, "", ""), nil
		},
		func(time.Duration) { sleeps++ },
	)
	defer restore()

	if err := node.ceosPostDeploy(context.Background()); err != nil {
		t.Fatalf("ceosPostDeploy() unexpected error: %v", err)
	}

	if calls != 3 {
		t.Fatalf("exec calls = %d, want 3", calls)
	}
	if configCalls != 1 {
		t.Fatalf("configuration calls = %d, want 1", configCalls)
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

func TestCeosPostDeployReturnsConfigurationErrorImmediately(t *testing.T) {
	node := newTestCEOSNode()

	var calls int
	restore := stubCeosPostDeploy(
		func(_ *ceos, _ context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
			calls++
			if isCeosReadinessCommand(execCmd) {
				return execResult(execCmd, 0, "", ""), nil
			}
			return execResult(execCmd, 1, "partial output", "syntax error"), nil
		},
		func(time.Duration) {},
	)
	defer restore()

	err := node.ceosPostDeploy(context.Background())
	if err == nil {
		t.Fatal("ceosPostDeploy() error = nil, want non-nil")
	}
	if calls != 2 {
		t.Fatalf("exec calls = %d, want one readiness check and one configuration attempt", calls)
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

func isCeosReadinessCommand(execCmd *clabexec.ExecCmd) bool {
	cmd := execCmd.GetCmd()
	return len(cmd) == 5 &&
		cmd[0] == "Cli" &&
		cmd[1] == "-p" &&
		cmd[2] == "15" &&
		cmd[3] == "-c" &&
		cmd[4] == "show version"
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

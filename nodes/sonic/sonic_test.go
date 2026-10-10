// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package sonic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablinks "github.com/srl-labs/containerlab/links"
	clabmocksmockruntime "github.com/srl-labs/containerlab/mocks/mockruntime"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	"go.uber.org/mock/gomock"
)

func TestWireInterfaceCmds(t *testing.T) {
	tests := map[string]struct {
		ifNames []string
		want    []string
	}{
		"no interfaces": {
			ifNames: nil,
			want:    []string{},
		},
		"skips the management interface": {
			ifNames: []string{"eth0", "eth1"},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
			},
		},
		"two interfaces": {
			ifNames: []string{"eth1", "eth2"},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
				"ip link set arp off dev eth2",
				"sysctl -w net.ipv6.conf.eth2.disable_ipv6=1",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := wireInterfaceCmds(tc.ifNames)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("wireInterfaceCmds() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// execOutcome decides what the runtime returns for a quieting command, so a
// test can make the wire fixups fail the way a real node can.
type execOutcome func(*clabexec.ExecCmd) (*clabexec.ExecResult, error)

// newTestNode returns a sonic node wired to a mock runtime, with one endpoint
// per name in ifNames, and the commands the runtime is asked to execute.
func newTestNode(
	t *testing.T,
	ifNames []string,
	outcome execOutcome,
) (*sonic, *[]string) {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockRt := clabmocksmockruntime.NewMockContainerRuntime(ctrl)

	n := &sonic{}
	n.DefaultNode = *clabnodes.NewDefaultNode(n)
	n.Cfg = &clabtypes.NodeConfig{ShortName: "s1", LongName: "clab-test-s1"}
	n.WithRuntime(mockRt)

	for _, ifName := range ifNames {
		n.Endpoints = append(n.Endpoints,
			clablinks.NewEndpointVeth(clablinks.NewEndpointGeneric(n, ifName, nil)))
	}

	got := &[]string{}

	mockRt.EXPECT().
		Exec(gomock.Any(), "clab-test-s1", gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ string,
			cmd *clabexec.ExecCmd,
		) (*clabexec.ExecResult, error) {
			*got = append(*got, cmd.GetCmdString())

			return outcome(cmd)
		}).
		AnyTimes()

	return n, got
}

func execOK(cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	return clabexec.NewExecResult(cmd), nil
}

// TestPostDeployEndpointsQuietsWires drives the wire fixups through the hook
// containerlab calls once the links exist.
func TestPostDeployEndpointsQuietsWires(t *testing.T) {
	tests := map[string]struct {
		ifNames []string
		outcome execOutcome
		want    []string
	}{
		"quiets every wire": {
			ifNames: []string{"eth1", "eth2"},
			outcome: execOK,
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
				"ip link set arp off dev eth2",
				"sysctl -w net.ipv6.conf.eth2.disable_ipv6=1",
			},
		},
		"leaves the management interface alone": {
			ifNames: []string{"eth0", "eth1"},
			outcome: execOK,
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
			},
		},
		"a node with no links runs nothing": {
			ifNames: nil,
			outcome: execOK,
			want:    []string{},
		},
		// A wire we cannot quiet is a warning, not a failed deploy - and it must
		// not cost the remaining wires their fixups.
		"an exec error does not stop the remaining wires": {
			ifNames: []string{"eth1", "eth2"},
			outcome: func(*clabexec.ExecCmd) (*clabexec.ExecResult, error) {
				return nil, errors.New("exec failed")
			},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
				"ip link set arp off dev eth2",
				"sysctl -w net.ipv6.conf.eth2.disable_ipv6=1",
			},
		},
		"a non-zero return code does not stop the remaining wires": {
			ifNames: []string{"eth1", "eth2"},
			outcome: func(cmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
				res := clabexec.NewExecResult(cmd)
				res.SetReturnCode(1)
				res.SetStdErr([]byte("no such device"))

				return res, nil
			},
			want: []string{
				"ip link set arp off dev eth1",
				"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
				"ip link set arp off dev eth2",
				"sysctl -w net.ipv6.conf.eth2.disable_ipv6=1",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			n, got := newTestNode(t, tc.ifNames, tc.outcome)

			if err := n.PostDeployEndpoints(context.Background()); err != nil {
				t.Fatalf("PostDeployEndpoints() returned %v, want nil", err)
			}

			if diff := cmp.Diff(tc.want, *got); diff != "" {
				t.Errorf("executed commands mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStartQuietsWires covers the Start override: starting a stopped node moves
// its endpoints back out of the parking namespace, which rebuilds their IPv6
// configuration, so the fixups have to run again.
func TestStartQuietsWires(t *testing.T) {
	n, got := newTestNode(t, []string{"eth1"}, execOK)

	rt, ok := n.GetRuntime().(*clabmocksmockruntime.MockContainerRuntime)
	if !ok {
		t.Fatalf("runtime is %T, want a mock runtime", n.GetRuntime())
	}
	// already running, so DefaultNode.Start returns before touching the runtime
	// any further and only the fixups are left to observe.
	rt.EXPECT().
		GetContainerStatus(gomock.Any(), "clab-test-s1").
		Return(clabruntime.Running)

	if err := n.Start(context.Background()); err != nil {
		t.Fatalf("Start() returned %v, want nil", err)
	}

	want := []string{
		"ip link set arp off dev eth1",
		"sysctl -w net.ipv6.conf.eth1.disable_ipv6=1",
	}
	if diff := cmp.Diff(want, *got); diff != "" {
		t.Errorf("executed commands mismatch (-want +got):\n%s", diff)
	}
}

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"bytes"
	"context"
	"errors"

	osexec "os/exec"

	containerlabexec "github.com/srl-labs/containerlab/exec"
	containerlablabels "github.com/srl-labs/containerlab/labels"
	containerlabnodes "github.com/srl-labs/containerlab/nodes"
	containerlabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	containerlabruntime "github.com/srl-labs/containerlab/runtime"
	containerlabtypes "github.com/srl-labs/containerlab/types"
	containerlabutils "github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"host"}

// Register registers the node in the NodeRegistry.
func Register(r *containerlabnodes.NodeRegistry) {
	r.Register(kindnames, func() containerlabnodes.Node {
		return new(host)
	}, nil)
}

type host struct {
	containerlabnodes.DefaultNode
}

func (n *host) Init(cfg *containerlabtypes.NodeConfig, opts ...containerlabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *containerlabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	n.Cfg.IsRootNamespaceBased = true
	return nil
}

func (n *host) Deploy(_ context.Context, _ *containerlabnodes.DeployParams) error {
	n.SetState(containerlabnodesstate.Deployed)
	return nil
}

func (*host) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*host) PullImage(_ context.Context) error             { return nil }
func (*host) Delete(_ context.Context) error                { return nil }
func (*host) WithMgmtNet(*containerlabtypes.MgmtNet)        {}

// UpdateConfigWithRuntimeInfo is a noop for hosts.
func (*host) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers returns a basic skeleton of a container to enable graphing of hosts kinds.
func (h *host) GetContainers(_ context.Context) ([]containerlabruntime.GenericContainer, error) {
	image := containerlabutils.GetOSRelease()

	return []containerlabruntime.GenericContainer{
		{
			Names:   []string{"Host"},
			State:   "running",
			ID:      "N/A",
			ShortID: "N/A",
			Image:   image,
			Labels: map[string]string{
				containerlablabels.NodeKind: kindnames[0],
			},
			Status: "running",
			NetworkSettings: containerlabruntime.GenericMgmtIPs{
				IPv4addr: "",
				// IPv4pLen: 0,
				IPv4Gw:   "",
				IPv6addr: "",
				// IPv6pLen: 0,
				IPv6Gw: "",
			},
		},
	}, nil
}

// RunExec runs commands on the container host.
func (*host) RunExec(ctx context.Context, e *containerlabexec.ExecCmd) (*containerlabexec.ExecResult, error) {
	return RunExec(ctx, e)
}

func RunExec(ctx context.Context, e *containerlabexec.ExecCmd) (*containerlabexec.ExecResult, error) {
	// retrieve the command with its arguments
	command := e.GetCmd()

	// execute the command along with the context
	cmd := osexec.CommandContext(ctx, command[0], command[1:]...)

	// create buffers for the output (stdout/stderr)
	var outBuf, errBuf bytes.Buffer

	// connect stdout and stderr to the buffers
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	// execute the command synchronously
	err := cmd.Run()
	exerr := &osexec.ExitError{}
	if err != nil && !errors.As(err, &exerr) {
		return nil, err
	}

	// create result struct
	execResult := containerlabexec.NewExecResult(e)
	// set the result fields in the exec struct
	execResult.SetReturnCode(cmd.ProcessState.ExitCode())
	execResult.SetStdOut(outBuf.Bytes())
	execResult.SetStdErr(errBuf.Bytes())

	return execResult, nil
}

// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"bytes"
	"context"
	"errors"

	osexec "os/exec"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabnodes "github.com/srl-labs/containerlab/nodes"
	clabnodesstate "github.com/srl-labs/containerlab/nodes/state"
	clabruntime "github.com/srl-labs/containerlab/runtime"
	clabtypes "github.com/srl-labs/containerlab/types"
	clabutils "github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"host"}

// Register registers the node in the NodeRegistry.
func Register(r *clabnodes.NodeRegistry) {
	r.Register(kindnames, func() clabnodes.Node {
		return new(host)
	}, nil)
}

type host struct {
	clabnodes.DefaultNode
}

func (n *host) Init(cfg *clabtypes.NodeConfig, opts ...clabnodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *clabnodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	n.Cfg.IsRootNamespaceBased = true
	return nil
}

func (n *host) Deploy(_ context.Context, _ *clabnodes.DeployParams) error {
	n.SetState(clabnodesstate.Deployed)
	return nil
}

func (*host) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*host) PullImage(_ context.Context) error             { return nil }
func (*host) Delete(_ context.Context) error                { return nil }
func (*host) WithMgmtNet(*clabtypes.MgmtNet)                {}

// UpdateConfigWithRuntimeInfo is a noop for hosts.
func (*host) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers returns a basic skeleton of a container to enable graphing of hosts kinds.
func (h *host) GetContainers(_ context.Context) ([]clabruntime.GenericContainer, error) {
	image := clabutils.GetOSRelease()

	return []clabruntime.GenericContainer{
		{
			Names:   []string{"Host"},
			State:   "running",
			ID:      clabconstants.NotApplicable,
			ShortID: clabconstants.NotApplicable,
			Image:   image,
			Labels: map[string]string{
				clabconstants.NodeKind: kindnames[0],
			},
			Status: "running",
			NetworkSettings: clabruntime.GenericMgmtIPs{
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
func (*host) RunExec(ctx context.Context, e *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	return RunExec(ctx, e)
}

func RunExec(ctx context.Context, e *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
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
	execResult := clabexec.NewExecResult(e)
	// set the result fields in the exec struct
	execResult.SetReturnCode(cmd.ProcessState.ExitCode())
	execResult.SetStdOut(outBuf.Bytes())
	execResult.SetStdErr(errBuf.Bytes())

	return execResult, nil
}

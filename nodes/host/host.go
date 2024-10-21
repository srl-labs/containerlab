// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package host

import (
	"bytes"
	"context"

	osexec "os/exec"

	cExec "github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/labels"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"host"}

// Register registers the node in the NodeRegistry.
func Register(r *nodes.NodeRegistry) {
	r.Register(kindnames, func() nodes.Node {
		return new(host)
	}, nil)
}

type host struct {
	nodes.DefaultNode
}

func (n *host) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	// Init DefaultNode
	n.DefaultNode = *nodes.NewDefaultNode(n)

	n.Cfg = cfg
	for _, o := range opts {
		o(n)
	}
	n.Cfg.IsRootNamespaceBased = true
	return nil
}

func (n *host) Deploy(_ context.Context, _ *nodes.DeployParams) error {
	n.SetState(state.Deployed)
	return nil
}

func (*host) GetImages(_ context.Context) map[string]string { return map[string]string{} }
func (*host) PullImage(_ context.Context) error             { return nil }
func (*host) Delete(_ context.Context) error                { return nil }
func (*host) WithMgmtNet(*types.MgmtNet)                    {}

// UpdateConfigWithRuntimeInfo is a noop for hosts.
func (*host) UpdateConfigWithRuntimeInfo(_ context.Context) error { return nil }

// GetContainers returns a basic skeleton of a container to enable graphing of hosts kinds.
func (h *host) GetContainers(_ context.Context) ([]runtime.GenericContainer, error) {
	image := utils.GetOSRelease()

	return []runtime.GenericContainer{
		{
			Names:   []string{"Host"},
			State:   "running",
			ID:      "N/A",
			ShortID: "N/A",
			Image:   image,
			Labels: map[string]string{
				labels.NodeKind: kindnames[0],
			},
			Status: "running",
			NetworkSettings: runtime.GenericMgmtIPs{
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
func (*host) RunExec(ctx context.Context, e *cExec.ExecCmd) (*cExec.ExecResult, error) {
	return RunExec(ctx, e)
}

func RunExec(ctx context.Context, e *cExec.ExecCmd) (*cExec.ExecResult, error) {
	// retireve the command with its arguments
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
	if err != nil {
		return nil, err
	}

	// create result struct
	execResult := cExec.NewExecResult(e)
	// set the result fields in the exec struct
	execResult.SetReturnCode(cmd.ProcessState.ExitCode())
	execResult.SetStdOut(outBuf.Bytes())
	execResult.SetStdErr(errBuf.Bytes())

	return execResult, nil
}

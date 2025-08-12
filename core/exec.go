package core

import (
	"context"
	"fmt"

	containerlabexec "github.com/srl-labs/containerlab/exec"
	containerlablinks "github.com/srl-labs/containerlab/links"
)

// Exec execute commands on running topology nodes.
func (c *CLab) Exec(ctx context.Context, cmds []string, listOptions ...ListOption) (*containerlabexec.ExecCollection, error) {
	err := containerlablinks.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	containers, err := c.ListContainers(ctx, listOptions...)
	if err != nil {
		return nil, err
	}

	// make sure filter returned containers
	if len(containers) == 0 {
		return nil, fmt.Errorf("filter did not match any containers")
	}

	// prepare the exec collection and the exec command
	resultCollection := containerlabexec.NewExecCollection()

	// build execs from the string input
	var execCmds []*containerlabexec.ExecCmd
	for _, execCmdStr := range cmds {
		execCmd, err := containerlabexec.NewExecCmdFromString(execCmdStr)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	// run the exec commands on all the containers matching the filter
	for idx := range containers {
		// iterate over the commands
		for _, execCmd := range execCmds {
			// execute the commands
			execResult, err := containers[idx].RunExec(ctx, execCmd)
			if err != nil {
				// skip nodes that do not support exec
				if err == containerlabexec.ErrRunExecNotSupported {
					continue
				}
			}

			resultCollection.Add(containers[idx].Names[0], execResult)
		}
	}

	return resultCollection, nil
}

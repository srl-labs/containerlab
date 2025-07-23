package core

import (
	"context"
	"fmt"

	"github.com/srl-labs/containerlab/exec"
	"github.com/srl-labs/containerlab/links"
)

// Exec execute commands on running topology nodes.
func (c *CLab) Exec(ctx context.Context, cmds []string, options *ExecOptions) (*exec.ExecCollection, error) {
	err := links.SetMgmtNetUnderlyingBridge(c.Config.Mgmt.Bridge)
	if err != nil {
		return nil, err
	}

	cnts, err := c.ListContainers(ctx, options.filters)
	if err != nil {
		return nil, err
	}

	// make sure filter returned containers
	if len(cnts) == 0 {
		return nil, fmt.Errorf("filter did not match any containers")
	}

	// prepare the exec collection and the exec command
	resultCollection := exec.NewExecCollection()

	// build execs from the string input
	var execCmds []*exec.ExecCmd
	for _, execCmdStr := range cmds {
		execCmd, err := exec.NewExecCmdFromString(execCmdStr)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	// run the exec commands on all the containers matching the filter
	for _, cnt := range cnts {
		// iterate over the commands
		for _, execCmd := range execCmds {
			// execute the commands
			execResult, err := cnt.RunExec(ctx, execCmd)
			if err != nil {
				// skip nodes that do not support exec
				if err == exec.ErrRunExecNotSupported {
					continue
				}
			}

			resultCollection.Add(cnt.Names[0], execResult)
		}
	}

	return resultCollection, nil
}

package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	clabexec "github.com/srl-labs/containerlab/exec"
	clablabruntime "github.com/srl-labs/containerlab/labruntime"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

// Exec execute commands on running topology nodes.
func (c *CLab) Exec(
	ctx context.Context,
	cmds []string,
	listOptions ...ListOption,
) (*clabexec.ExecCollection, error) {
	if c.LabRuntime != nil {
		return c.execWithLabRuntime(ctx, cmds, listOptions...)
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
	resultCollection := clabexec.NewExecCollection()

	// build execs from the string input
	var execCmds []*clabexec.ExecCmd
	for _, execCmdStr := range cmds {
		execCmd, err := clabexec.NewExecCmdFromString(execCmdStr)
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
				if errors.Is(err, clabexec.ErrRunExecNotSupported) {
					continue
				}
				log.Warnf("exec on %s failed: %v", containers[idx].Names[0], err)
				continue
			}

			resultCollection.Add(containers[idx].Names[0], execResult)
		}
	}

	return resultCollection, nil
}

// ExecAttached runs a command in the container of the one node the list options select, with
// stdin, an optional TTY and terminal resizing, like docker exec -it. A command exiting nonzero
// returns a *labruntime.CommandExitError, a command that can't start, such as a missing
// executable, returns an error without writing any output.
func (c *CLab) ExecAttached(
	ctx context.Context,
	cmd []string,
	tty bool,
	streams clablabruntime.AttachStreams,
	listOptions ...ListOption,
) error {
	if len(cmd) == 0 {
		return errors.New("an attached exec needs a command")
	}

	if c.LabRuntime != nil {
		containers, err := c.ListLabRuntimeContainers(ctx, false, listOptions...)
		if err != nil {
			return err
		}

		target, err := attachedExecTarget(containers)
		if err != nil {
			return err
		}

		namespace, labName, nodeName, err := labRuntimeContainerParts(*target)
		if err != nil {
			return fmt.Errorf("exec target %s is invalid: %w", target.Names[0], err)
		}

		return c.AttachLabRuntimeNode(ctx, clablabruntime.AttachRequest{
			Name:      labName,
			Namespace: namespace,
			NodeName:  nodeName,
			Command:   cmd,
			TTY:       tty,
		}, streams)
	}

	containers, err := c.ListContainers(ctx, listOptions...)
	if err != nil {
		return err
	}

	target, err := attachedExecTarget(containers)
	if err != nil {
		return err
	}

	attacher, ok := target.Runtime.(clabruntime.ContainerAttacher)
	if !ok {
		return fmt.Errorf("container runtime %q does not support attached exec",
			target.Runtime.GetName())
	}

	return attacher.AttachContainer(ctx, target.Names[0], cmd, tty, streams)
}

// attachedExecTarget returns the container of an attached exec, which needs the filter to match
// exactly one.
func attachedExecTarget(
	containers []clabruntime.GenericContainer,
) (*clabruntime.GenericContainer, error) {
	switch len(containers) {
	case 0:
		return nil, fmt.Errorf("filter did not match any containers")
	case 1:
		return &containers[0], nil
	}

	return nil, fmt.Errorf(
		"an attached exec needs a filter matching exactly one container, the filter matched %d",
		len(containers))
}

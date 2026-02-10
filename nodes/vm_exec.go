package nodes

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/crypto/ssh"

	clabexec "github.com/srl-labs/containerlab/exec"
)

// RunVMExec executes a command inside a VM guest by SSHing from the containerlab
// host directly to the VM's management interface. This is used for vrnetlab-based
// VM nodes where the standard docker exec runs in the QEMU wrapper container,
// not inside the guest VM.
//
// The SSH connection targets the container's long name (resolved via Docker DNS)
// on port 22, which vrnetlab forwards to the guest VM.
func RunVMExec(ctx context.Context, addr, username, password string,
	execCmd *clabexec.ExecCmd,
) (*clabexec.ExecResult, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         10 * time.Second,
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", addr), config)
	if err != nil {
		return nil, fmt.Errorf("failed to SSH to VM guest %s: %w", addr, err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session for %s: %w", addr, err)
	}
	defer session.Close()

	cmd := strings.Join(execCmd.GetCmd(), " ")

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	result := clabexec.NewExecResult(execCmd)

	err = session.Run(cmd)

	result.SetStdOut(stdout.Bytes())
	result.SetStdErr(stderr.Bytes())

	if err != nil {
		// If the command exited with a non-zero status, capture it
		// but don't return an error -- the caller can check the return code.
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.SetReturnCode(exitErr.ExitStatus())
			log.Debugf("VM exec on %s returned exit code %d: %s", addr, exitErr.ExitStatus(), cmd)
		} else {
			return nil, fmt.Errorf("failed to execute command on VM guest %s: %w", addr, err)
		}
	}

	return result, nil
}

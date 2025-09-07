package runtime

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	clabconstants "github.com/srl-labs/containerlab/constants"
	clabexec "github.com/srl-labs/containerlab/exec"
	clabtypes "github.com/srl-labs/containerlab/types"
)

// GenericContainer stores generic container data.
type GenericContainer struct {
	Names           []string
	ID              string
	ShortID         string // trimmed ID for display purposes
	Image           string
	State           string
	Status          string
	Labels          map[string]string
	Pid             int
	NetworkName     string
	NetworkSettings GenericMgmtIPs
	Mounts          []ContainerMount
	Runtime         ContainerRuntime
	Ports           []*clabtypes.GenericPortBinding
}

type ContainerMount struct {
	Source      string
	Destination string
}

// SetRuntime sets the runtime for this GenericContainer.
func (ctr *GenericContainer) SetRuntime(r ContainerRuntime) {
	ctr.Runtime = r
}

// RunExec executes a single command for a GenericContainer.
func (gc *GenericContainer) RunExec(ctx context.Context, execCmd *clabexec.ExecCmd) (*clabexec.ExecResult, error) {
	containerName := gc.Names[0]
	execResult, err := gc.Runtime.Exec(ctx, containerName, execCmd)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v", containerName, execCmd.GetCmdString(), err)
		return nil, err
	}
	return execResult, nil
}

func (ctr *GenericContainer) GetContainerIPv4() string {
	if ctr.NetworkSettings.IPv4addr == "" {
		return clabconstants.NotApplicable
	}
	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv4addr, ctr.NetworkSettings.IPv4pLen)
}

func (ctr *GenericContainer) GetContainerIPv6() string {
	if ctr.NetworkSettings.IPv6addr == "" {
		return clabconstants.NotApplicable
	}
	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv6addr, ctr.NetworkSettings.IPv6pLen)
}

type GenericMgmtIPs struct {
	IPv4addr string
	IPv4pLen int
	IPv4Gw   string
	IPv6addr string
	IPv6pLen int
	IPv6Gw   string
}

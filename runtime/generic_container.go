package runtime

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/types"
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
	NetworkSettings GenericMgmtIPs
	Mounts          []ContainerMount
	runtime         ContainerRuntime
	Ports           []*types.GenericPortBinding
}

type ContainerMount struct {
	Source      string
	Destination string
}

// SetRuntime sets the runtime for this GenericContainer.
func (ctr *GenericContainer) SetRuntime(r ContainerRuntime) {
	ctr.runtime = r
}

// RunExec executes a single command for a GenericContainer.
func (gc *GenericContainer) RunExec(ctx context.Context, execCmd *exec.ExecCmd) (*exec.ExecResult, error) {
	containerName := gc.Names[0]
	execResult, err := gc.runtime.Exec(ctx, containerName, execCmd)
	if err != nil {
		log.Errorf("%s: failed to execute cmd: %q with error %v", containerName, execCmd.GetCmdString(), err)
		return nil, err
	}
	return execResult, nil
}

// // RunExecTypeWoWait is the final function that calls the runtime to execute a type.Exec on a GenericContainer
// func (gc *GenericContainer) RunExecTypeWoWait(ctx context.Context, execCmd *exec.ExecCmd) error {
// 	containerName := gc.Names[0]
// 	err := gc.runtime.ExecNotWait(ctx, containerName, execCmd)
// 	if err != nil {
// 		log.Errorf("%s: failed to execute cmd: %q with error %v", containerName, execCmd.GetCmdString(), err)
// 		return err
// 	}
// 	return nil
// }

func (ctr *GenericContainer) GetContainerIPv4() string {
	if ctr.NetworkSettings.IPv4addr == "" {
		return "N/A"
	}
	return fmt.Sprintf("%s/%d", ctr.NetworkSettings.IPv4addr, ctr.NetworkSettings.IPv4pLen)
}

func (ctr *GenericContainer) GetContainerIPv6() string {
	if ctr.NetworkSettings.IPv6addr == "" {
		return "N/A"
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

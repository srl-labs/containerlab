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
	Config          GenericContainerConfig
}

type ContainerMount struct {
	Source      string
	Destination string
}

// GenericContainerConfig stores inspectable container-create settings that apply can compare
// against desired node config without using persistent labels or state files.
type GenericContainerConfig struct {
	Available     bool
	Image         string
	User          string
	Entrypoint    []string
	Cmd           []string
	Env           map[string]string
	Labels        map[string]string
	Binds         []string
	ExposedPorts  []*clabtypes.GenericPortBinding
	PortBindings  []*clabtypes.GenericPortBinding
	NetworkMode   string
	PidMode       string
	MacAddress    string
	Aliases       []string
	ExtraHosts    []string
	Sysctls       map[string]string
	DNS           *clabtypes.DNSConfig
	CapAdd        []string
	Devices       []string
	Tmpfs         map[string]string
	ShmSize       int64
	CPUQuota      int64
	CPUPeriod     int64
	CPUSet        string
	Memory        int64
	RestartPolicy string
	Healthcheck   *clabtypes.HealthcheckConfig

	// UncomparableFields lists inspect fields the runtime cannot expose reliably.
	UncomparableFields map[string]struct{}
}

// SetRuntime sets the runtime for this GenericContainer.
func (ctr *GenericContainer) SetRuntime(r ContainerRuntime) {
	ctr.Runtime = r
}

// RunExec executes a single command for a GenericContainer.
func (gc *GenericContainer) RunExec(
	ctx context.Context,
	execCmd *clabexec.ExecCmd,
) (*clabexec.ExecResult, error) {
	containerName := gc.Names[0]
	execResult, err := gc.Runtime.Exec(ctx, containerName, execCmd)
	if err != nil {
		log.Errorf(
			"%s: failed to execute cmd: %q with error %v",
			containerName,
			execCmd.GetCmdString(),
			err,
		)
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

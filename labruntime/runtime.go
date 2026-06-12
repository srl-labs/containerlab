package labruntime

import (
	"context"
	"fmt"
	"time"

	clabexec "github.com/srl-labs/containerlab/exec"
)

const (
	ClabernetesRuntimeName = "clabernetes"
)

type Config struct {
	Debug   bool
	Timeout time.Duration
}

type DeployRequest struct {
	Name               string
	Namespace          string
	Owner              string
	TopologyFile       string
	TopologyLabDir     string
	TopologyDefinition []byte
	Wait               bool
	Timeout            time.Duration
}

type DestroyRequest struct {
	Name      string
	Namespace string
	Wait      bool
	Timeout   time.Duration
}

type InspectRequest struct {
	Name      string
	Namespace string
}

type ListRequest struct {
	Namespace     string
	AllNamespaces bool
}

type NodeRequest struct {
	Name      string
	Namespace string
	Nodes     []string
	Timeout   time.Duration
}

type ExecRequest struct {
	Name      string
	Namespace string
	NodeName  string
	Command   []string
}

type SaveRequest struct {
	Name      string
	Namespace string
	Nodes     []string
	Copy      bool
}

type EventStreamRequest struct {
	Namespace             string
	AllNamespaces         bool
	IncludeInitialState   bool
	IncludeInterfaceStats bool
	StatsInterval         time.Duration
}

type SavedFile struct {
	NodeName   string
	Name       string
	Data       []byte
	Mode       int64
	LinkTarget string
}

type SaveResult struct {
	Files []SavedFile
}

type NodeState struct {
	Name                string
	Kind                string
	Image               string
	State               string
	Ready               bool
	LoadBalancerAddress string
}

type LabState struct {
	Name         string
	Namespace    string
	Owner        string
	TopologyPath string
	State        string
	Ready        bool
	Nodes        []NodeState
}

type Event struct {
	Timestamp   time.Time
	Type        string
	Action      string
	ActorID     string
	ActorName   string
	ActorFullID string
	Attributes  map[string]string
}

type LabRuntime interface {
	Deploy(context.Context, DeployRequest) (*LabState, error)
	Destroy(context.Context, DestroyRequest) error
	Inspect(context.Context, InspectRequest) (*LabState, error)
	List(context.Context, ListRequest) ([]*LabState, error)
	Exec(context.Context, ExecRequest) (*clabexec.ExecResult, error)
	Start(context.Context, NodeRequest) error
	Stop(context.Context, NodeRequest) error
	Restart(context.Context, NodeRequest) error
	Save(context.Context, SaveRequest) (*SaveResult, error)
	StreamEvents(context.Context, EventStreamRequest) (<-chan Event, <-chan error, error)
}

type Initializer func(Config) (LabRuntime, error)

var LabRuntimes = map[string]Initializer{}

func Register(name string, init Initializer) {
	LabRuntimes[name] = init
}

func IsLabRuntimeName(name string) bool {
	_, ok := LabRuntimes[name]
	return ok
}

func Init(name string, cfg Config) (LabRuntime, error) {
	init, ok := LabRuntimes[name]
	if !ok {
		return nil, fmt.Errorf("unknown lab runtime %q", name)
	}

	return init(cfg)
}

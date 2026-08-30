package labruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	clabexec "github.com/srl-labs/containerlab/exec"
)

const (
	ClabernetesRuntimeName = "c9s"
	// DefaultImagePullSecret is the image pull secret name populated in the clabernetes
	// Topology when a deploy request does not name one.
	DefaultImagePullSecret = "regcred"
)

type Config struct {
	Timeout   time.Duration
	Namespace string
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
	// NoTopologyCR deploys without a controller-owned topology object: the runtime compiles the
	// topology client-side and manages the individual lab resources directly.
	NoTopologyCR bool
	// ImagePullSecret names the same-namespace Docker-config Secret placed on device pods
	// through Pod.spec.imagePullSecrets. Runtimes fall back to DefaultImagePullSecret when it
	// is empty.
	ImagePullSecret string
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
	Name  string
	Kind  string
	Image string
	State string
	Ready bool
	// MgmtIPv4Address and MgmtIPv6Address are the node's allocated management addresses in
	// CIDR notation, when the runtime reports them.
	MgmtIPv4Address     string
	MgmtIPv6Address     string
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

type ChangeAction string

const (
	ChangeCreate ChangeAction = "create"
	ChangeUpdate ChangeAction = "update"
	ChangeDelete ChangeAction = "delete"
)

type ResourceChange struct {
	Action    ChangeAction `json:"action"`
	Kind      string       `json:"kind"`
	Namespace string       `json:"namespace,omitempty"`
	Name      string       `json:"name"`
}

// DeployPlan describes the remote resources a lab runtime would change without mutating them.
type DeployPlan struct {
	LabName   string           `json:"lab-name"`
	Namespace string           `json:"namespace"`
	Changes   []ResourceChange `json:"changes"`
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

// LabExistenceChecker lets controller-driven runtimes report whether a lab has remote state.
// Core uses it to distinguish a fresh deployment from reconciliation without consulting the
// local container runtime.
type LabExistenceChecker interface {
	LabExists(context.Context, InspectRequest) (bool, error)
}

// TopologyValidator validates the runtime-specific topology subset without changing remote state.
type TopologyValidator interface {
	Validate(context.Context, DeployRequest) error
}

// TopologyPlanner compiles and diffs a desired topology without changing remote state.
type TopologyPlanner interface {
	Plan(context.Context, DeployRequest) (*DeployPlan, error)
}

type Initializer func(Config) (LabRuntime, error)

var LabRuntimes = map[string]Initializer{}

func Register(name string, init Initializer) {
	LabRuntimes[name] = init
}

func IsLabRuntimeName(name string) bool {
	_, ok := LabRuntimes[strings.ToLower(name)]
	return ok
}

func Init(name string, cfg Config) (LabRuntime, error) {
	name = strings.ToLower(name)
	init, ok := LabRuntimes[name]
	if !ok {
		return nil, fmt.Errorf("unknown lab runtime %q", name)
	}

	return init(cfg)
}

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

	ExposeTypeClusterIP    = "ClusterIP"
	ExposeTypeHeadless     = "Headless"
	ExposeTypeLoadBalancer = "LoadBalancer"
	ExposeTypeNone         = "None"
)

var exposeTypes = map[string]string{ //nolint:gochecknoglobals
	strings.ToLower(ExposeTypeClusterIP):    ExposeTypeClusterIP,
	strings.ToLower(ExposeTypeHeadless):     ExposeTypeHeadless,
	strings.ToLower(ExposeTypeLoadBalancer): ExposeTypeLoadBalancer,
	strings.ToLower(ExposeTypeNone):         ExposeTypeNone,
}

// NormalizeExposeType validates a c9s expose type and returns its canonical CRD spelling.
func NormalizeExposeType(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if normalized, ok := exposeTypes[strings.ToLower(value)]; ok {
		return normalized, nil
	}

	return "", fmt.Errorf("invalid expose type %q; must be one of %s, %s, %s, %s",
		value,
		ExposeTypeClusterIP,
		ExposeTypeHeadless,
		ExposeTypeLoadBalancer,
		ExposeTypeNone,
	)
}

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
	// through Pod.spec.imagePullSecrets. Empty means the lab references no pull secret at
	// all: public images and clusters whose runtime already holds credentials need none.
	ImagePullSecret string
	// ExposeType controls the Kubernetes Service type c9s creates for lab nodes. Empty uses the
	// c9s CRD default.
	ExposeType string
	// NoPersistence deploys the lab on ephemeral node storage. By default every node's
	// artifact volume is backed by a persistent claim so saved device configuration survives
	// Pod replacement, matching the local-runtime lab directory contract; opting out trades
	// that for not needing a dynamically provisionable storage class, and every Pod
	// replacement then resets the node to its declared configuration.
	NoPersistence bool
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

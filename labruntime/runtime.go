package labruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"time"

	clabexec "github.com/srl-labs/containerlab/exec"
	clabtypes "github.com/srl-labs/containerlab/types"
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

// Identity is the Kubernetes identity a lab runtime acts as, through user impersonation. The
// zero value acts with the configured credentials (the kubeconfig) as they are. A server that
// runs labs for many users sets it per call, so the cluster's RBAC and audit log see the user.
type Identity struct {
	User   string
	Groups []string
}

// IsZero reports whether the identity leaves the configured credentials as they are.
func (i Identity) IsZero() bool {
	return i.User == "" && len(i.Groups) == 0
}

// Equal reports whether two identities impersonate the same user and groups.
func (i Identity) Equal(other Identity) bool {
	return i.User == other.User && slices.Equal(i.Groups, other.Groups)
}

type Config struct {
	Timeout   time.Duration
	Namespace string
	// Identity is the identity of every call that does not set its own.
	Identity Identity

	// KindLinkApplyMode resolves the default link apply mode ("live", "restart" or
	// "recreate") a node kind declares, as containerlab's node registry reports it. An
	// empty result means the kind is unknown and the conservative recreate default
	// applies. Lab runtimes use it to predict the node lifecycle impact of link changes.
	KindLinkApplyMode func(kind string) string
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
	// MaxWorkers limits concurrent independent operations performed by the runtime.
	MaxWorkers uint
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
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

type DestroyRequest struct {
	Name       string
	Namespace  string
	Wait       bool
	Timeout    time.Duration
	MaxWorkers uint
	// Cleanup also deletes the lab's persistent node storage (saved configuration). Without
	// it the storage is kept, like the lab directory of a local runtime, and a later deploy
	// of the lab boots from it.
	Cleanup bool
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

type InspectRequest struct {
	Name      string
	Namespace string
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

// ListRequest selects the labs List reports. A Namespace, or the runtime's configured
// namespace override, limits the listing to that namespace, even with AllNamespaces.
type ListRequest struct {
	Namespace     string
	AllNamespaces bool
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

type NodeRequest struct {
	Name      string
	Namespace string
	Nodes     []string
	Timeout   time.Duration
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

type ExecRequest struct {
	Name      string
	Namespace string
	NodeName  string
	Command   []string
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

type SaveRequest struct {
	Name      string
	Namespace string
	Nodes     []string
	Copy      bool
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
}

// EventStreamRequest selects the events StreamEvents reports. Name limits the stream to one
// lab, in its namespace. Without Name, Namespace or the runtime's configured namespace override
// limit it to one namespace; AllNamespaces otherwise covers every namespace.
type EventStreamRequest struct {
	Name                  string
	Namespace             string
	AllNamespaces         bool
	IncludeInitialState   bool
	IncludeInterfaceStats bool
	StatsInterval         time.Duration
	// Identity overrides the runtime's configured identity for this call.
	Identity Identity
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

// Container states and health in the vocabulary of container runtimes (Docker), which inspect
// and events report for lab runtime nodes too.
const (
	ContainerStateCreated = "created"
	ContainerStateRunning = "running"
	ContainerStateExited  = "exited"

	ContainerHealthHealthy   = "healthy"
	ContainerHealthStarting  = "starting"
	ContainerHealthUnhealthy = "unhealthy"
)

type NodeState struct {
	Name string
	// TopologyName is the node name as the topology file spells it, when the runtime had to
	// carry the node under another name (Name). Empty means Name.
	TopologyName string
	Kind         string
	Image        string
	// Type and Group are the node's type and group when the topology set them.
	Type  string
	Group string
	// State is the runtime's own view of the node, such as the clabernetes Node readiness, or
	// stopped.
	State string
	Ready bool
	// ContainerState is the node's state in container runtime terms: created, running or
	// exited (the ContainerState* constants). Empty when the runtime does not report one.
	ContainerState string
	// Health is the device health of a running node in container runtime terms: healthy or
	// starting. Empty when the node does not run.
	Health string
	// MgmtIPv4Address and MgmtIPv6Address are the node's allocated management addresses in
	// CIDR notation, when the runtime reports them.
	MgmtIPv4Address     string
	MgmtIPv6Address     string
	LoadBalancerAddress string
	// Pod is the pod that runs the node. Nodes that share a network namespace share a pod.
	Pod string
	// Container is the node's primary device container in Pod.
	Container string
	// NodeUID identifies the runtime's node resource (the clabernetes Node UID).
	NodeUID string
	// ExposedPorts are the node ports the runtime publishes through a Service.
	ExposedPorts []ExposedPort
}

// ExposedPort is a node port a lab runtime publishes through a Service.
type ExposedPort = clabtypes.ExposedPort

type LabState struct {
	Name      string
	Namespace string
	Owner     string
	// TopologyPath is the absolute path of the topology file the lab was deployed from, on the
	// machine that deployed it. A lab the runtime did not deploy from a file reports where the
	// runtime keeps its definition instead (for c9s k8s://<namespace>/topologies/<lab>).
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
	// ChangeRecreate marks a resource that is deleted and created again, such as the Pod
	// of a node whose links change. It is not a Kubernetes action; it communicates the
	// lifecycle impact of an apply.
	ChangeRecreate ChangeAction = "recreate"
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
	// Attributes are the runtime's own attributes. Event consumers add the attributes the
	// container runtimes report from Lab, Node and Interface.
	Attributes map[string]string
	// Lab and Node are the lab node a container or interface event is about, as Inspect
	// reports it. containerlab's events command describes the node from them the same way
	// inspect does, so both agree and match what the container runtimes report.
	Lab  *LabState
	Node *NodeState
	// Interface is the interface an interface event reports.
	Interface *InterfaceEventData
}

// Event types and the origin attribute of lab runtime events.
const (
	EventTypeContainer = "container"
	EventTypeInterface = "interface"
	// EventOriginSnapshot marks the events of --initial-state, in the origin attribute.
	EventOriginSnapshot = "snapshot"
)

// InterfaceEventData describes an interface of a node the way the container runtimes' interface
// events do (netlink).
type InterfaceEventData struct {
	// Name is the interface name inside the node.
	Name string
	// Alias is the name the topology gave the interface when the node kind maps it (for
	// example ethernet-1/1 for SR Linux e1-1).
	Alias string
	Index int
	MTU   int
	MAC   string
	// OperState is the operational state in netlink's vocabulary: up, down, lower-layer-down,
	// dormant, testing, not-present or unknown.
	OperState string
	// Type is the link type in netlink's vocabulary, such as veth or device (loopback).
	Type string
	// Stats are set on stats events.
	Stats *InterfaceStats
}

// InterfaceStats are the counters and rates of an interface stats event.
type InterfaceStats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxBps     float64
	TxBps     float64
	RxPps     float64
	TxPps     float64
	Interval  time.Duration
}

// NodeLocation is the workload that runs a lab node.
type NodeLocation struct {
	// NodeName is the node name the runtime carries (the sanitized name).
	NodeName  string
	Namespace string
	Pod       string
	// Container is the node's primary device container in Pod.
	Container string
	// NodeUID identifies the runtime's node resource (the clabernetes Node UID).
	NodeUID string
	// ExposedPorts are the node ports the runtime publishes through a Service.
	ExposedPorts        []ExposedPort
	LoadBalancerAddress string
}

// TerminalSize is the size of an interactive terminal, in characters.
type TerminalSize struct {
	Width  uint16
	Height uint16
}

// AttachRequest selects the node and the command of an interactive session.
type AttachRequest struct {
	// Name is the lab name.
	Name      string
	Namespace string
	// NodeName is the node to attach to, as the topology or the runtime names it.
	NodeName string
	// Container overrides the node's primary device container.
	Container string
	// Command runs in the container; empty runs "sh".
	Command []string
	// TTY allocates a terminal. Stdout then carries the combined output, and Resize applies.
	TTY      bool
	Identity Identity
}

// AttachStreams connects an interactive session. Nil streams are not attached.
type AttachStreams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Resize delivers terminal size changes of a TTY session; closing it stops resizing.
	Resize <-chan TerminalSize
}

// CommandExitError reports a command that ran and exited with a nonzero code.
type CommandExitError struct {
	Code int
}

func (e *CommandExitError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.Code)
}

// LogRequest selects the node log StreamLogs writes.
type LogRequest struct {
	// Name is the lab name.
	Name      string
	Namespace string
	// NodeName is the node whose log is streamed, as the topology or the runtime names it.
	NodeName string
	// Container overrides the node's primary device container.
	Container string
	// Tail limits the output to the last lines of the log; nil writes the whole log.
	Tail *int64
	// Follow keeps streaming new log lines until the context ends.
	Follow bool
	// Timestamps prefixes every line with its timestamp.
	Timestamps bool
	Identity   Identity
}

// NodeStats is a resource usage sample of a node.
type NodeStats struct {
	NodeName  string
	Namespace string
	Pod       string
	// CPUMilliCores is the CPU usage of the node's device containers, in thousandths of a core.
	CPUMilliCores int64
	// MemoryBytes is the memory working set of the node's device containers.
	MemoryBytes int64
	// Timestamp and Window describe the sampling interval the usage was measured over.
	Timestamp time.Time
	Window    time.Duration
}

// ErrMetricsUnavailable reports a cluster without a resource metrics API (metrics.k8s.io), for
// example one without metrics-server.
var ErrMetricsUnavailable = errors.New("metrics API (metrics.k8s.io) not available")

// ErrUnsupportedRuntimeVersion reports a cluster whose controller release lacks the contract a
// lab runtime needs. The error that wraps it names the required release.
var ErrUnsupportedRuntimeVersion = errors.New("unsupported lab runtime controller version")

// SupportChecker checks that the cluster behind a lab runtime runs a controller release with
// the contract the runtime needs. Lab runtimes run the check on their own before the first
// cluster call of an operation; servers call it to fail early, for example at startup. It returns
// an error wrapping ErrUnsupportedRuntimeVersion when the release is too old.
type SupportChecker interface {
	CheckSupport(ctx context.Context, identity Identity) error
}

// NodeLocator resolves lab nodes to the workloads that run them. Empty NodeRequest.Nodes
// locates every node of the lab.
type NodeLocator interface {
	LocateNode(ctx context.Context, req NodeRequest) ([]NodeLocation, error)
}

// NodeAttacher runs an interactive command (stdin, TTY and resize) in a node's container. It
// returns a *CommandExitError when the command exits with a nonzero code.
type NodeAttacher interface {
	Attach(ctx context.Context, req AttachRequest, streams AttachStreams) error
}

// NodeLogStreamer writes a node container's log.
type NodeLogStreamer interface {
	StreamLogs(ctx context.Context, req LogRequest, w io.Writer) error
}

// NodePortForwarder opens a stream to a TCP port of a node (SSH, web UIs). The request must name
// exactly one node. Closing the connection closes the stream.
type NodePortForwarder interface {
	DialPort(ctx context.Context, req NodeRequest, port int) (net.Conn, error)
}

// CaptureInterface is a lab link interface packet capture accepts, with the far end of its link.
type CaptureInterface struct {
	// Node is the node name as the topology spells it.
	Node string `json:"node"`
	// Interface is the interface name inside the node, which capture output carries.
	Interface string `json:"interface"`
	// Alias is the name the topology used for the interface when the node kind maps it (for
	// example ethernet-1/1 for SR Linux e1-1). Capture targets accept either name.
	Alias string `json:"alias,omitempty"`
	// PeerNode is the node at the other end of the link, "host" for a host endpoint, or empty
	// when the runtime cannot name it.
	PeerNode string `json:"peer_node"`
	// PeerInterface is the peer's interface as the topology wrote it.
	PeerInterface string `json:"peer_interface"`
}

// CaptureTarget is one interface of one node to capture. Node takes the name the topology or
// the runtime uses, Interface the interface name or its alias.
type CaptureTarget struct {
	Node      string
	Interface string
}

// CaptureRequest selects what Capture streams.
type CaptureRequest struct {
	// Name is the lab name.
	Name      string
	Namespace string
	Targets   []CaptureTarget
	// Duration ends the capture; zero captures until the context ends.
	Duration time.Duration
	// PacketLimit ends the capture after that many packets across all targets; zero means no
	// limit.
	PacketLimit int
	// SnapLength is the number of bytes kept of each packet; zero uses the runtime default.
	SnapLength int
	// OnGap is called when the capture missed traffic, for example because a chained capture
	// did not start in time. Without it, gaps are logged as warnings.
	OnGap    func(CaptureGap)
	Identity Identity
}

// CaptureGap is a time span a capture has no packets for, although it was running.
type CaptureGap struct {
	Node       string
	Interfaces []string
	// From and To bound the gap on the node's clock; To is zero when it is unknown.
	From   time.Time
	To     time.Time
	Reason string
}

// PacketCapturer streams pcapng from lab link interfaces. Capture writes one section with one
// Interface Description Block per target, named <node>:<interface>, before any packet. It
// returns when Duration passed, PacketLimit packets were written, or the context ended (which
// is not an error); the output is a valid capture whenever it stops.
type PacketCapturer interface {
	CaptureInterfaces(ctx context.Context, req NodeRequest) ([]CaptureInterface, error)
	Capture(ctx context.Context, req CaptureRequest, w io.Writer) error
}

// InterfaceInspector lists the link interfaces of a lab's nodes with their current state, in the
// shape of `inspect interfaces`.
type InterfaceInspector interface {
	InspectInterfaces(ctx context.Context, req NodeRequest) ([]*clabtypes.ContainerInterfaces, error)
}

// NodeStatsSampler reports the resource usage of a lab's nodes. It returns an error wrapping
// ErrMetricsUnavailable when the cluster serves no resource metrics.
type NodeStatsSampler interface {
	SampleStats(ctx context.Context, req InspectRequest) ([]NodeStats, error)
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

// Manifest is one remote resource a lab runtime would create for a topology, ready to be
// serialized and applied by hand. Object is the complete resource including its type and
// metadata; the identity fields are duplicated for summaries and tests.
type Manifest struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Object     map[string]any
}

// ManifestEmitter renders the exact resources a lab runtime would create for a topology, in
// apply order, without reading or changing remote state.
type ManifestEmitter interface {
	Manifests(context.Context, DeployRequest) ([]Manifest, error)
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

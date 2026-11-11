// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

// Package backend defines the data model of the containerlab TUI and the interface the UI
// uses to read lab state and to run operations. The UI never talks to a container runtime
// directly, which keeps it testable against a fake.
package backend

import (
	"context"
	"errors"
	"time"
)

// ErrLabBusy is returned when an operation is requested for a lab that already has a
// state-changing job running.
var ErrLabBusy = errors.New("another operation is already running for this lab")

// ErrUnavailable is returned when an operation cannot run in the current environment, ie.
// because root privileges cannot be obtained.
var ErrUnavailable = errors.New("operation is not available")

// Topic identifies a part of the state. Topics are combined into a bit set to tell the UI
// what changed.
type Topic uint32

// State topics.
const (
	// TopicLabs covers the lab and node inventory, including node state.
	TopicLabs Topic = 1 << iota
	// TopicStats covers the sampled values: traffic rates, CPU and memory.
	TopicStats
	// TopicInterfaces covers the interface inventory, its state and impairments.
	TopicInterfaces
	// TopicLinks covers the topology of a lab.
	TopicLinks
	// TopicJobs covers the job list and the state of the jobs.
	TopicJobs
	// TopicJobOutput covers new output lines of a job.
	TopicJobOutput
	// TopicMessages covers the messages logged by the backend itself.
	TopicMessages
	// TopicHost covers the host information and the capabilities.
	TopicHost
)

// TopicAll matches every topic.
const TopicAll = ^Topic(0)

// Has reports whether t includes any of the topics in other.
func (t Topic) Has(other Topic) bool { return t&other != 0 }

// Backend is the UI's view of containerlab. Implementations are safe for concurrent use.
//
// The backend owns the state: it watches the runtime, keeps the inventory current and
// signals changes through Notify. The UI then reads what it needs with the getters, which
// return copies that are safe to keep.
type Backend interface {
	// Start begins watching the runtime. It returns once the first inventory is loaded.
	Start(ctx context.Context) error
	// Close stops the watchers and cancels the running jobs and streams.
	Close()

	// Notify returns a channel that receives a value whenever state has changed since
	// the last call to Changed.
	Notify() <-chan struct{}
	// Changed returns the topics that changed since the previous call and resets them.
	Changed() Topic
	// SetFocus tells the backend what is on screen, so it can limit sampling to it.
	SetFocus(f Focus)
	// Refresh requests an immediate reload of the inventory.
	Refresh()

	Host() Host
	// Labs returns the running labs and the topology files that are not deployed.
	Labs() []Lab
	// Interfaces returns the interfaces of a node.
	Interfaces(node NodeRef) []Interface
	// Topology returns the nodes and links of a lab.
	Topology(lab string) Topology
	// Messages returns the messages logged by the backend.
	Messages() []LogLine

	// Logs streams the log of a node.
	Logs(ctx context.Context, node NodeRef, opts LogOptions) (LogStream, error)
	// Exec runs a command in the nodes and sends one result per node. The channel is
	// closed once every node has answered or ctx is done.
	Exec(ctx context.Context, nodes []NodeRef, command string) (<-chan ExecResult, error)
	// Inspect returns a detailed, human readable description of a node.
	Inspect(ctx context.Context, node NodeRef) (string, error)

	// ConnectMethods lists the ways to open an interactive session on a node, the
	// preferred one first.
	ConnectMethods(node *Node) []ConnectMethod
	// Interactive prepares an interactive session. The caller hands the terminal over to
	// it.
	Interactive(node NodeRef, method ConnectMethod) (Session, error)

	// The operations below change state. They run as jobs and return at once.
	Deploy(req DeployRequest) (JobID, error)
	Destroy(req DestroyRequest) (JobID, error)
	Redeploy(req RedeployRequest) (JobID, error)
	NodeAction(req NodeActionRequest) (JobID, error)
	Save(req SaveRequest) (JobID, error)
	// SetNetem and ResetNetem work on interfaces of one lab, all of them in a single job.
	SetNetem(req NetemRequest) (JobID, error)
	ResetNetem(interfaces []InterfaceRef) (JobID, error)
	Share(req ShareRequest) (JobID, error)
	Capture(req CaptureRequest) (JobID, error)

	// Jobs returns the jobs, the most recent first.
	Jobs() []JobInfo
	// Job returns a single job.
	Job(id JobID) (JobInfo, bool)
	// JobOutput returns the output lines of a job starting at the given line number.
	JobOutput(id JobID, from int) []LogLine
	// CancelJob asks a running job to stop.
	CancelJob(id JobID)

	// Packets returns the packets of a live capture starting at the given position, which
	// counts the packets from zero in the order they arrived. The oldest packets of a
	// capture that saw more than is kept are gone.
	Packets(id JobID, from int) []Packet
	// PacketDetail returns everything known about one packet of a live capture, by its
	// number.
	PacketDetail(id JobID, number int) (PacketDetail, bool)
	// RemoteCapture returns the command line that runs a capture from the computer of a
	// user who works on this host through SSH, and shows it in the Wireshark there.
	RemoteCapture(req CaptureRequest) (string, error)
}

// Focus describes what the UI currently shows.
type Focus struct {
	// Lab is the lab on screen, empty when every lab is shown.
	Lab string
	// Node is the node whose interfaces are on screen, if any.
	Node string
	// Rates is set while traffic rates are visible.
	Rates bool
}

// Host describes the environment the TUI runs in.
type Host struct {
	Runtime  string
	Version  string
	Hostname string
	User     string
	// Privileged reports whether operations that need root privileges can run.
	Privileged bool
	// Demo is set when the data is simulated.
	Demo bool
	// Cluster is set when the labs run in a Kubernetes cluster rather than on this host:
	// the nodes are pods then, and what works on the containers and the network of this
	// host does not apply.
	Cluster bool
	// SSH is how the computer of the user reaches this host, as the arguments of the ssh
	// command. It is empty when the UI is not used through SSH.
	SSH  string
	Caps Capabilities
}

// Capabilities tells which groups of operations are available.
type Capabilities struct {
	// Lifecycle covers deploy, destroy, redeploy and the node start, stop and restart.
	Lifecycle bool
	Netem     bool
	Capture   bool
	Share     bool
	// LinkState is set when the operational state of interfaces is known.
	LinkState bool
	// Wireshark is set when a local Wireshark can be started.
	Wireshark bool
	// Reasons explains why a capability is off, keyed by the lower case field name.
	Reasons map[string]string
}

// Reason returns why the named capability is unavailable.
func (c *Capabilities) Reason(name string) string {
	if r, ok := c.Reasons[name]; ok {
		return r
	}

	return "not available"
}

// NodeRef identifies a node.
type NodeRef struct {
	// Lab is the name of the lab the node belongs to.
	Lab string
	// Name is the node name as written in the topology.
	Name string
	// Container is the name of the node's container.
	Container string
}

// Lab is a deployed lab or a topology file that is not deployed.
type Lab struct {
	Name string
	// Namespace is the Kubernetes namespace of a lab that runs in a cluster.
	Namespace string
	// TopoFile is the absolute path of the topology file, empty when unknown.
	TopoFile string
	LabDir   string
	Owner    string
	Deployed bool
	// TopoMissing is set for a deployed lab whose topology file no longer exists.
	TopoMissing bool
	// Busy is set while a state-changing job runs for the lab.
	Busy  bool
	Nodes []Node
	// DefinedNodes is the number of nodes in the topology file, zero when unknown.
	DefinedNodes int
	// Shares are the terminal sharing sessions attached to the lab.
	Shares []Share
}

// LabState summarizes the state of the nodes of a lab.
type LabState int

// Lab states.
const (
	LabUndeployed LabState = iota
	LabRunning
	LabPartial
	LabStopped
	LabFailed
)

func (s LabState) String() string {
	switch s {
	case LabRunning:
		return "running"
	case LabPartial:
		return "partial"
	case LabStopped:
		return "stopped"
	case LabFailed:
		return "failed"
	default:
		return "undeployed"
	}
}

// State returns the aggregated state of the lab.
func (l *Lab) State() LabState {
	if !l.Deployed {
		return LabUndeployed
	}

	var ready, running, failed int

	for i := range l.Nodes {
		switch l.Nodes[i].Readiness() {
		case Ready:
			ready++
			running++
		case Starting, Unhealthy, Paused, Stopping:
			running++
		case Failed:
			failed++
		case Stopped, Unknown:
		}
	}

	switch {
	case len(l.Nodes) > 0 && ready == len(l.Nodes):
		return LabRunning
	case failed > 0 && running == 0:
		return LabFailed
	case running > 0:
		return LabPartial
	default:
		return LabStopped
	}
}

// ReadyNodes returns the number of nodes that are up and healthy.
func (l *Lab) ReadyNodes() int {
	n := 0

	for i := range l.Nodes {
		if l.Nodes[i].Readiness() == Ready {
			n++
		}
	}

	return n
}

// Node finds a node of the lab by its name.
func (l *Lab) Node(name string) *Node {
	for i := range l.Nodes {
		if l.Nodes[i].Name == name {
			return &l.Nodes[i]
		}
	}

	return nil
}

// Port is a published or exposed container port.
type Port struct {
	HostIP        string
	HostPort      int
	ContainerPort int
	Protocol      string
}

// Node is a lab node backed by a container.
type Node struct {
	NodeRef
	ID    string
	Kind  string
	Type  string
	Group string
	Image string
	// State is the runtime's container state, ie. running or exited.
	State string
	// Status is the runtime's status text, ie. "Up 5 minutes (healthy)".
	Status string
	IPv4   string
	IPv6   string
	Ports  []Port
	Owner  string
	Pid    int
	Labels map[string]string

	// Sampled values. HasStats is false until the first sample was taken.
	HasStats   bool
	CPUPercent float64
	MemBytes   uint64
	MemLimit   uint64
	RxBps      float64
	TxBps      float64
	// CPUSeries, RxSeries and TxSeries hold the recent samples, the oldest first.
	CPUSeries []float64
	RxSeries  []float64
	TxSeries  []float64
}

// Ref returns the reference of the node.
func (n *Node) Ref() NodeRef { return n.NodeRef }

// Readiness tells how far a node is from being usable.
type Readiness int

// Readiness values.
const (
	Unknown Readiness = iota
	Ready
	Starting
	Unhealthy
	Paused
	Stopping
	Stopped
	Failed
)

func (r Readiness) String() string {
	switch r {
	case Ready:
		return "ready"
	case Starting:
		return "starting"
	case Unhealthy:
		return "unhealthy"
	case Paused:
		return "paused"
	case Stopping:
		return "stopping"
	case Stopped:
		return "stopped"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Running reports whether the node's container runs, healthy or not.
func (r Readiness) Running() bool {
	return r == Ready || r == Starting || r == Unhealthy
}

// Interface is a network interface of a node.
type Interface struct {
	Node  NodeRef
	Name  string
	Alias string
	MAC   string
	Type  string
	MTU   int
	Index int
	// State is "up", "down" or empty when unknown.
	State string
	// Peer is the far end of the link the interface belongs to, if any.
	Peer  *Endpoint
	Netem *Impairment

	HasStats  bool
	RxBps     float64
	TxBps     float64
	RxPps     float64
	TxPps     float64
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxSeries  []float64
	TxSeries  []float64
}

// Impairment holds the link impairments set on an interface.
type Impairment struct {
	Delay         time.Duration
	Jitter        time.Duration
	LossPct       float64
	RateKbit      uint64
	CorruptionPct float64
}

// IsZero reports whether no impairment is set.
func (i *Impairment) IsZero() bool {
	return i == nil || *i == Impairment{}
}

// Endpoint is one end of a link.
type Endpoint struct {
	Node      string
	Interface string
	MAC       string
	// External is set for endpoints that are not lab nodes, ie. the host or a bridge.
	External bool
}

// Link connects two endpoints. Z is nil for links with a single end.
type Link struct {
	A Endpoint
	Z *Endpoint
}

// TopoNode is a node as it appears in a topology.
type TopoNode struct {
	Name  string
	Kind  string
	Group string
	// Role is the node's function, ie. spine or leaf, when the topology states it.
	Role string
	// Level is the tier of the node in the topology drawing, zero when unknown.
	Level int
	// X and Y are the saved coordinates of the node in a topology drawing.
	X, Y   float64
	HasPos bool
}

// Topology holds the nodes and links of a lab.
type Topology struct {
	Lab   string
	Nodes []TopoNode
	Links []Link
	// Warning explains why the topology is missing or incomplete.
	Warning string
}

// Share is a terminal sharing session attached to a lab.
type Share struct {
	Tool ShareTool
	Lab  string
	Link string
	// ReadOnlyLink is the view-only link, when the tool offers one.
	ReadOnlyLink string
}

// ShareTool names a terminal sharing tool.
type ShareTool string

// Terminal sharing tools.
const (
	ShareSSHX  ShareTool = "sshx"
	ShareGoTTY ShareTool = "gotty"
)

// LogLine is a line of output.
type LogLine struct {
	Time time.Time
	// Stderr is set for lines written to the standard error stream.
	Stderr bool
	// Level is the severity found in the line, ie. "INFO", empty when there is none.
	Level string
	Text  string
}

// LogOptions selects what a log stream returns.
type LogOptions struct {
	// Tail is the number of lines to start with, a negative value returns everything.
	Tail       int
	Follow     bool
	Timestamps bool
}

// LogStream delivers log lines in batches.
type LogStream interface {
	// Lines returns the channel the batches arrive on. It is closed when the stream
	// ends, after which Err tells why.
	Lines() <-chan []LogLine
	Err() error
	Close()
}

// ExecResult is the outcome of a command run in a node.
type ExecResult struct {
	Node     NodeRef
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	Duration time.Duration
}

// ConnectMethod is a way to open an interactive session on a node.
type ConnectMethod struct {
	Label       string
	Description string
	// Shell is the command to run in the container. It is unused for SSH.
	Shell []string
	SSH   bool
}

// JobID identifies a job.
type JobID int

// JobKind tells what a job does.
type JobKind string

// Job kinds.
const (
	JobDeploy   JobKind = "deploy"
	JobDestroy  JobKind = "destroy"
	JobRedeploy JobKind = "redeploy"
	JobStart    JobKind = "start"
	JobStop     JobKind = "stop"
	JobRestart  JobKind = "restart"
	JobSave     JobKind = "save"
	JobNetem    JobKind = "netem"
	JobShare    JobKind = "share"
	JobCapture  JobKind = "capture"
)

// JobState is the state of a job.
type JobState int

// Job states.
const (
	JobRunning JobState = iota
	JobCancelling
	JobSucceeded
	JobFailed
	JobCancelled
)

func (s JobState) String() string {
	switch s {
	case JobRunning:
		return "running"
	case JobCancelling:
		return "cancelling"
	case JobSucceeded:
		return "done"
	case JobFailed:
		return "failed"
	case JobCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Done reports whether the job has finished.
func (s JobState) Done() bool {
	return s == JobSucceeded || s == JobFailed || s == JobCancelled
}

// JobInfo describes a job.
type JobInfo struct {
	ID    JobID
	Kind  JobKind
	Lab   string
	Title string
	// Command is the equivalent command line, with secrets removed.
	Command  string
	State    JobState
	ExitCode int
	// Err is the error the job ended with, empty on success.
	Err      string
	Started  time.Time
	Finished time.Time
	Lines    int
	Warnings int
	Errors   int
	// Bytes is the amount of data a capture has written so far.
	Bytes uint64
	// Live is set for a capture whose packets are decoded for the UI, Packets is how many
	// of them arrived so far.
	Live    bool
	Packets int
}

// Duration returns how long the job has been running, or ran.
func (j *JobInfo) Duration(now time.Time) time.Duration {
	if j.Finished.IsZero() {
		return now.Sub(j.Started)
	}

	return j.Finished.Sub(j.Started)
}

// DeployRequest describes a lab deployment.
type DeployRequest struct {
	Lab      string
	TopoFile string
	// Reconfigure regenerates the lab directory of an already deployed lab.
	Reconfigure    bool
	MaxWorkers     int
	NodeFilter     []string
	SkipPostDeploy bool
}

// DestroyRequest describes the removal of a lab.
type DestroyRequest struct {
	Lab      string
	TopoFile string
	// Cleanup also removes the lab directory.
	Cleanup     bool
	Graceful    bool
	KeepMgmtNet bool
}

// RedeployRequest describes a destroy followed by a deploy.
type RedeployRequest struct {
	Lab         string
	TopoFile    string
	Cleanup     bool
	Graceful    bool
	KeepMgmtNet bool
	MaxWorkers  int
}

// NodeAction is a lifecycle action on nodes.
type NodeAction string

// Node lifecycle actions.
const (
	NodeStart   NodeAction = "start"
	NodeStop    NodeAction = "stop"
	NodeRestart NodeAction = "restart"
)

// NodeActionRequest describes a lifecycle action on nodes of one lab. An empty node list
// selects every node of the lab.
type NodeActionRequest struct {
	Lab    string
	Action NodeAction
	Nodes  []string
}

// SaveRequest describes saving the configuration of nodes. An empty node list selects
// every node of the lab.
type SaveRequest struct {
	Lab   string
	Nodes []string
	// CopyTo is a directory the saved configurations are also copied to.
	CopyTo string
}

// InterfaceRef identifies an interface of a node.
type InterfaceRef struct {
	Node NodeRef
	Name string
}

// NetemRequest sets the impairments of interfaces of one lab. It replaces whatever was set
// on them before.
type NetemRequest struct {
	Interfaces []InterfaceRef
	Impairment
}

// ShareAction is an action on a terminal sharing session.
type ShareAction string

// Terminal sharing actions.
const (
	ShareAttach   ShareAction = "attach"
	ShareDetach   ShareAction = "detach"
	ShareReattach ShareAction = "reattach"
)

// ShareRequest describes an action on a terminal sharing session of a lab.
type ShareRequest struct {
	Lab    string
	Tool   ShareTool
	Action ShareAction
	// ReadOnly asks sshx for an additional view-only link.
	ReadOnly bool
	// Port, Username, Password and Shell configure a GoTTY session.
	Port     int
	Username string
	Password string
	Shell    string
}

// CaptureTarget selects interfaces of a node to capture on.
type CaptureTarget struct {
	Node       string
	Interfaces []string
}

// CaptureRequest describes a packet capture.
type CaptureRequest struct {
	Lab      string
	TopoFile string
	Targets  []CaptureTarget
	// Wireshark opens the capture in a local Wireshark and Live decodes it for the UI,
	// which reads the packets with Packets. Otherwise the capture is written to File. A
	// live capture is written to File as well when one is given.
	Wireshark bool
	Live      bool
	File      string
	// Duration and PacketLimit stop the capture, zero means no limit.
	Duration    time.Duration
	PacketLimit int
}

// Packet is a captured packet, summed up in one line.
type Packet struct {
	// Number counts the packets of a capture from one.
	Number int
	Time   time.Time
	// Interface is where the packet was captured, as node:interface.
	Interface string
	// Source and Destination are the addresses of the highest layer that has some.
	Source      string
	Destination string
	// Protocol is the name of the highest protocol that was recognized.
	Protocol string
	// Length is the size of the packet on the wire, in bytes.
	Length int
	Info   string
}

// PacketField is a named value of a protocol header.
type PacketField struct {
	Name  string
	Value string
}

// PacketLayer is one protocol header of a packet.
type PacketLayer struct {
	Name   string
	Fields []PacketField
}

// PacketDetail is a packet with its protocol headers, the outermost first.
type PacketDetail struct {
	Packet
	Layers []PacketLayer
	// Data holds the bytes of the packet that were kept, Captured tells how many there
	// were. A capture keeps the start of large packets only.
	Data     []byte
	Captured int
}

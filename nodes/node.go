// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/srl-labs/containerlab/cert"
	"github.com/srl-labs/containerlab/clab/exec"
	"github.com/srl-labs/containerlab/links"
	"github.com/srl-labs/containerlab/nodes/state"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/vishvananda/netlink"
	"golang.org/x/crypto/ssh"
)

const (
	// default connection mode for vrnetlab based containers.
	VrDefConnMode = "tc"
	// keys for the map returned by GetImages.
	ImageKey   = "image"
	KernelKey  = "kernel"
	SandboxKey = "sandbox"
)

var (
	// a map of node kinds overriding the default global runtime.
	NonDefaultRuntimes = map[string]string{}

	// ErrCommandExecError is an error returned when a command is failed to execute on a given node.
	ErrCommandExecError = errors.New("command execution error")
	// ErrContainersNotFound indicated that for a given node no containers where found in the runtime.
	ErrContainersNotFound = errors.New("containers not found")
	// ErrNoStartupConfig indicates that we are supposed to enforce a startup config but none are provided
	ErrNoStartupConfig = errors.New("no startup-config provided")
	// ErrIncompatibleOptions for options that are mutually exclusive
	ErrIncompatibleOptions = errors.New("incompatible options")
)

// SetNonDefaultRuntimePerKind sets a non default runtime for kinds that requires that (see cvx).
func SetNonDefaultRuntimePerKind(kindnames []string, runtime string) error {
	for _, kindname := range kindnames {
		if _, exists := NonDefaultRuntimes[kindname]; exists {
			return fmt.Errorf("non default runtime config for kind with the name '%s' exists already", kindname)
		}
		NonDefaultRuntimes[kindname] = runtime
	}
	return nil
}

type PreDeployParams struct {
	Cert         *cert.Cert
	TopologyName string
	TopoPaths    *types.TopoPaths
	SSHPubKeys   []ssh.PublicKey
}

// DeployParams contains parameters for the Deploy function.
type DeployParams struct{}

// PostDeployParams contains parameters for the PostDeploy function.
type PostDeployParams struct {
	Nodes map[string]Node
}

// Node is an interface that defines the behavior of a node.
type Node interface {
	Init(*types.NodeConfig, ...NodeOption) error
	// GetContainers returns a pointer to GenericContainer that the node uses.
	GetContainers(ctx context.Context) ([]runtime.GenericContainer, error)
	DeleteNetnsSymlink() (err error)
	Config() *types.NodeConfig // Config returns the nodes configuration
	// CheckDeploymentConditions checks if node-scoped deployment conditions are met.
	CheckDeploymentConditions(context.Context) error
	PreDeploy(ctx context.Context, params *PreDeployParams) error
	Deploy(context.Context, *DeployParams) error // Deploy triggers the deployment of this node
	PostDeploy(ctx context.Context, params *PostDeployParams) error
	WithMgmtNet(*types.MgmtNet)           // WithMgmtNet provides the management network for the node
	WithRuntime(runtime.ContainerRuntime) // WithRuntime provides the runtime for the node
	// CalculateInterfaceIndex returns with the interface index offset from the first valid dataplane interface based on the interface name. Errors otherwise.
	CalculateInterfaceIndex(ifName string) (int, error)
	// CheckInterfaceName checks if a name of the interface referenced in the topology file is correct for this node
	CheckInterfaceName() error
	// VerifyStartupConfig checks for existence of the referenced file and maybe performs additional config checks
	VerifyStartupConfig(topoDir string) error
	SaveConfig(context.Context) error            // SaveConfig saves the nodes configuration to an external file
	Delete(context.Context) error                // Delete triggers the deletion of this node
	GetImages(context.Context) map[string]string // GetImages returns the images used for this kind
	GetRuntime() runtime.ContainerRuntime        // GetRuntime returns the nodes assigned runtime
	GenerateConfig(dst, templ string) error      // Generate the nodes configuration
	// UpdateConfigWithRuntimeInfo updates node config with runtime info like IP addresses assigned by runtime
	UpdateConfigWithRuntimeInfo(context.Context) error
	// RunExec execute a single command for a given node.
	RunExec(ctx context.Context, execCmd *exec.ExecCmd) (*exec.ExecResult, error)
	// Adds the given link to the Node (container). After adding the Link to the node,
	// the given function f is called within the Nodes namespace to setup the link.
	AddLinkToContainer(ctx context.Context, link netlink.Link, f func(ns.NetNS) error) error
	AddEndpoint(e links.Endpoint) error
	GetEndpoints() []links.Endpoint
	GetLinkEndpointType() links.LinkEndpointType
	GetShortName() string
	// DeployEndpoints deploys the links for the node.
	DeployEndpoints(ctx context.Context) error
	// ExecFunction executes the given function within the nodes network namespace
	ExecFunction(context.Context, func(ns.NetNS) error) error
	GetState() state.NodeState
	SetState(state.NodeState)
	GetSSHConfig() *types.SSHConfig
	// RunExecFromConfig executes the topologyfile defined exec commands
	RunExecFromConfig(context.Context, *exec.ExecCollection) error
	IsHealthy(ctx context.Context) (bool, error)
	GetContainerStatus(ctx context.Context) runtime.ContainerStatus
}

type NodeOption func(Node)

func WithMgmtNet(mgmt *types.MgmtNet) NodeOption {
	return func(n Node) {
		if mgmt == nil {
			n.WithMgmtNet(new(types.MgmtNet))
			return
		}
		n.WithMgmtNet(mgmt)
	}
}

func WithRuntime(r runtime.ContainerRuntime) NodeOption {
	return func(n Node) {
		n.WithRuntime(r)
	}
}

// GenericVMInterfaceCheck checks interface names for generic VM-based nodes.
// These nodes could only have interfaces named ethX, where X is >0.
func GenericVMInterfaceCheck(nodeName string, eps []links.Endpoint) error {
	ifRe := regexp.MustCompile(`eth[1-9][0-9]*$`) // skipcq: GO-C4007
	for _, e := range eps {
		if !ifRe.MatchString(e.GetIfaceName()) {
			return fmt.Errorf("%q interface name %q doesn't match the required pattern. It should be named as ethX, where X is >0", nodeName, e.GetIfaceName())
		}
	}

	return nil
}

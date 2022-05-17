// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"context"
	"fmt"

	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
)

const (
	// default connection mode for vrnetlab based containers
	VrDefConnMode = "tc"
	// keys for the map returned by GetImages
	ImageKey   = "image"
	KernelKey  = "kernel"
	SandboxKey = "sandbox"
)

var NodeKind string

const (
	NodeKindBridge = "bridge"

	NodeKindHOST = "host"
	NodeKindOVS  = "ovs-bridge"
	NodeKindSRL  = "srl"
)

// a map of node kinds overriding the default global runtime
var NonDefaultRuntimes = map[string]string{}

// SetNonDefaultRuntimePerKind sets a non default runtime for kinds that requires that (see cvx)
func SetNonDefaultRuntimePerKind(kindnames []string, runtime string) error {
	for _, kindname := range kindnames {
		if _, exists := NonDefaultRuntimes[kindname]; exists {
			return fmt.Errorf("non default runtime config for kind with the name '%s' exists already", kindname)
		}
		NonDefaultRuntimes[kindname] = runtime
	}
	return nil
}

type Node interface {
	Init(*types.NodeConfig, ...NodeOption) error
	Config() *types.NodeConfig
	PreDeploy(configName, labCADir, labCARoot string) error
	Deploy(context.Context) error
	PostDeploy(context.Context, map[string]Node) error
	WithMgmtNet(*types.MgmtNet)
	WithRuntime(runtime.ContainerRuntime)
	SaveConfig(context.Context) error
	Delete(context.Context) error
	GetImages() map[string]string
	GetRuntime() runtime.ContainerRuntime
}

// Nodes is a map of all supported kinds and their init functions
var Nodes = map[string]Initializer{}

type Initializer func() Node

func Register(names []string, initFn Initializer) {
	for _, name := range names {
		Nodes[name] = initFn
	}
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

var DefaultConfigTemplates = map[string]string{
	"vr-sros": "",
}

// DefaultCredentials holds default username and password per each kind
var defaultCredentials = map[string][]string{}

// SetDefaultCredentials register default credentials per provided kindname
func SetDefaultCredentials(kindnames []string, user, password string) error {
	// iterate over the kindnames
	for _, kindname := range kindnames {
		// check the default credentials for the kindname is not yet already registed
		if _, exists := defaultCredentials[kindname]; exists {
			return fmt.Errorf("kind with the name '%s' exists already", kindname)
		}
		// register the credentials
		defaultCredentials[kindname] = []string{user, password}
	}
	return nil
}

// GetDefaultCredentialsForKind retrieve the default credentials for a certain kind
// the first element in the slice is the Username, the second is the password
func GetDefaultCredentialsForKind(kind string) ([]string, error) {
	if _, exists := defaultCredentials[kind]; !exists {
		return nil, fmt.Errorf("default credentials entry for kind %s does not exist", kind)
	}
	return defaultCredentials[kind], nil
}

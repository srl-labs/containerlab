// Copyright 2020 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package nodes

import (
	"context"

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
	NodeKindBridge     = "bridge"
	NodeKindCEOS       = "ceos"
	NodeKindCVX        = "cvx"
	NodeKindCRPD       = "crpd"
	NodeKindHOST       = "host"
	NodeKindLinux      = "linux"
	NodeKindMySocketIO = "mysocketio"
	NodeKindOVS        = "bridge-ovs"
	NodeKindSonic      = "sonic"
	NodeKindSRL        = "srl"
	NodeKindVrCSR      = "vr-csr"
	NodeKindVrPAN      = "vr-pan"
	NodeKindVrROS      = "vr-ros"
	NodeKindVrSROS     = "vr-sros"
	NodeKindVrVEOS     = "vr-veos"
	NodeKindVrVMX      = "vr-vmx"
	NodeKindVrXRV      = "vr-xrv"
	NodeKindVrXRV9K    = "vr-xrv9k"
)

// a map of node kinds overriding the default global runtime
var NonDefaultRuntimes = map[string]string{
	NodeKindCVX: runtime.IgniteRuntime,
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

var Nodes = map[string]Initializer{}

type Initializer func() Node

func Register(name string, initFn Initializer) {
	Nodes[name] = initFn
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
	"crpd":    "/etc/containerlab/templates/crpd/juniper.conf",
	"vr-sros": "",
}

// DefaultCredentials holds default username and password per each kind
var DefaultCredentials = map[string][]string{
	"srl":      {"admin", "admin"},
	"vr-pan":  {"admin", "Admin@123"},
	"vr-sros":  {"admin", "admin"},
	"vr-vmx":   {"admin", "admin@123"},
	"vr-xrv9k": {"clab", "clab@123"},
}
